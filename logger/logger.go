package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/memory198/go-gear/logger/core"
)

// Config 日志配置
type Config struct {
	Level         Level  // 最低输出等级
	Format        Format // TextFormat（默认）/ JSONFormat
	Console       bool   // 是否输出到控制台（和文件可同时）
	FileDir       string // 文件输出目录，空则不写文件
	Filename      string // 文件名（不含扩展名），空则取程序名
	MaxAge        int    // 日志保留天数，<=0 不清理
	Caller        bool   // 是否记录一行调用位置（直接调用者，内部有缓存，非调用栈）
	ParentSpanIDs bool   // 是否输出 parent_span_ids（中间 span 链；默认关闭，开启需读取 gctx 聚合）

	// resource 元信息（写入日志的 resource 对象；留空则不输出对应键）
	Service string // service.name
	Version string // service.version
	Env     string // deployment.environment
	Host    string // host.name（空则尝试取 os.Hostname）

	// OnError 输出/滚动失败时的回调（可选）
	// 用于发现磁盘满、句柄失效等问题；回调在写入锁内被调用，应快速返回
	OnError func(error)
}

// Logger 日志实例
type Logger struct {
	cfg        Config                    // 日志配置（级别、格式、输出渠道等）
	mu         sync.Mutex                // 并发写入锁，保证日志行不交叉
	enc        encoder                   // 日志编码器（text 或 json）
	writers    []io.Writer               // 输出目标列表（stdout + 文件可同时存在）
	currentDay string                    // 当前日志文件所属日期，用于判断是否需要滚动
	file       *os.File                  // 当前打开的日志文件句柄，仅文件输出时非 nil
	res        Resource                  // resource 元信息（构造时确定，避免每行重复计算）
	hooks      atomic.Pointer[[]Emitter] // 结构化输出旁路（OTLP 等），空时热路径零开销
}

// New 创建 Logger
// Console=false && FileDir="" → 兜底输出到 stdout
func New(cfg Config) (*Logger, error) {
	l := &Logger{cfg: cfg}

	switch cfg.Format {
	case JSONFormat:
		l.enc = jsonEncoder{}
	default:
		l.enc = textEncoder{}
	}

	// resource 在构造时确定（host 缺省取本机主机名）
	l.res = cfg.Resource()

	if err := l.openWriters(); err != nil {
		return nil, err
	}
	return l, nil
}

// Resource 返回配置对应的 resource（host 缺省取 os.Hostname）
// 用于与 OTLP 等外部导出保持一致：otlpsink.WithResource(cfg.Resource())
func (c Config) Resource() Resource {
	r := Resource{
		ServiceName:    c.Service,
		ServiceVersion: c.Version,
		Environment:    c.Env,
		HostName:       c.Host,
	}
	if r.HostName == "" {
		r.HostName, _ = os.Hostname()
	}
	return r
}

// NewFromConfig 从配置参数创建
//
// Deprecated: 位置参数过多且难扩展，建议使用 New(Config{...})
func NewFromConfig(level, format, fileDir, filename string, console bool, maxAge int, caller bool) (*Logger, error) {
	return New(Config{
		Level:    parseLevel(level),
		Format:   parseFormat(format),
		Console:  console,
		FileDir:  fileDir,
		Filename: filename,
		MaxAge:   maxAge,
		Caller:   caller,
	})
}

// ---- 实例方法：不带格式化 ----

// 日志显示效果（text 格式，msg 后的键值对以 key=value 追加）：
// ctx 携带 root_trace_id 时输出 [abc123] 段（Print/Printf 无 ctx，无此段）
//
//	Info(ctx, "user created", "user_id", 123)
//	→ 2026-09-02 10:30:00.123456 [INFO] [abc123] handler/user.go:42 user created user_id=123
//
//	Debug(ctx, "cache hit")
//	→ 2026-09-02 10:30:00.123456 [DEBUG] [abc123] handler/user.go:42 cache hit
//
//	Warn(ctx, "slow query", "ms", 320)
//	→ 2026-09-02 10:30:00.123456 [WARN] [abc123] handler/user.go:42 slow query ms=320
//
//	Error(ctx, "db down")
//	→ 2026-09-02 10:30:00.123456 [ERROR] [abc123] handler/user.go:42 db down
//
//	Print("server starting", "port", 8080)
//	→ 2026-09-02 10:30:00.123456 [INFO] main.go:42 server starting port=8080
//
// JSON 格式（json 编码器）携带完整链路字段，键值对平铺为独立 JSON 字段：
//
//	Info(ctx, "user created", "user_id", 123)
//	→ {"time":"2026-09-02 10:30:00.123456","level":"INFO","msg":"user created",
//	   "caller":"handler/user.go:42","root_trace_id":"abc123",
//	   "middle_span_ids":["m1"],"current_span_id":"s2","user_id":123}

func (l *Logger) Debug(ctx context.Context, msg string, args ...any) { l.log(ctx, DEBUG, msg, args...) }
func (l *Logger) Info(ctx context.Context, msg string, args ...any)  { l.log(ctx, INFO, msg, args...) }
func (l *Logger) Warn(ctx context.Context, msg string, args ...any)  { l.log(ctx, WARN, msg, args...) }
func (l *Logger) Error(ctx context.Context, msg string, args ...any) { l.log(ctx, ERROR, msg, args...) }

// Fatal 输出 FATAL 等级日志后退出程序（调用 os.Exit(1)，defer 不会执行）
// 退出前会尝试刷新实现了 core.Flusher 的 Hook（如 OTLP 批量缓冲）
func (l *Logger) Fatal(ctx context.Context, msg string, args ...any) {
	l.log(ctx, FATAL, msg, args...)
	l.flushHooks(ctx)
	os.Exit(1)
}

// ---- 实例方法：带格式化 ----

func (l *Logger) Debugf(ctx context.Context, format string, args ...any) {
	l.log(ctx, DEBUG, fmt.Sprintf(format, args...))
}
func (l *Logger) Infof(ctx context.Context, format string, args ...any) {
	l.log(ctx, INFO, fmt.Sprintf(format, args...))
}
func (l *Logger) Warnf(ctx context.Context, format string, args ...any) {
	l.log(ctx, WARN, fmt.Sprintf(format, args...))
}
func (l *Logger) Errorf(ctx context.Context, format string, args ...any) {
	l.log(ctx, ERROR, fmt.Sprintf(format, args...))
}

// Fatalf 格式化 FATAL 等级日志后退出程序
// 退出前会尝试刷新实现了 core.Flusher 的 Hook
func (l *Logger) Fatalf(ctx context.Context, format string, args ...any) {
	l.log(ctx, FATAL, fmt.Sprintf(format, args...))
	l.flushHooks(ctx)
	os.Exit(1)
}

// flushHooks 刷新支持 core.Flusher 的 Hook（带短超时，避免拖住退出流程）
func (l *Logger) flushHooks(ctx context.Context) {
	hs := l.hooks.Load()
	if hs == nil {
		return
	}
	flushCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for _, h := range *hs {
		if f, ok := h.(core.Flusher); ok {
			_ = f.Flush(flushCtx)
		}
	}
}

// ---- 无 ctx 打印（log 风格） ----
// 面向启动、后台任务等非请求场景：内部使用 context.Background()，
// 因此不携带 trace 链路字段；请求内日志请使用 Info(ctx, ...) 等带 ctx 方法。

// Print 输出 INFO 级日志（log 风格，无 ctx）
// 参数语义：msg 为消息正文，args 为 slog 键值对字段（与 Info(ctx, msg, kv...) 一致）
// ⚠️ 不做格式化——需要格式化请用 Printf
func (l *Logger) Print(msg string, args ...any) {
	l.log(context.Background(), INFO, msg, args...)
}

// Printf 格式化输出 INFO 级日志（无 ctx）
// 参数语义：format + 格式化参数（内部 fmt.Sprintf），与 Print 的 kv 语义不同
func (l *Logger) Printf(format string, args ...any) {
	l.log(context.Background(), INFO, fmt.Sprintf(format, args...))
}

// AddHook 注册结构化日志消费点（如 OTLP 导出），在序列化之前对每条日志调用
// 线程安全（CAS 循环）；通常应在服务启动阶段注册
func (l *Logger) AddHook(h Emitter) {
	if h == nil {
		return
	}
	for {
		old := l.hooks.Load()
		var next []Emitter
		if old != nil {
			next = append(next, *old...)
		}
		next = append(next, h)
		if l.hooks.CompareAndSwap(old, &next) {
			return
		}
	}
}

// log 核心写入逻辑
func (l *Logger) log(ctx context.Context, level Level, msg string, args ...any) {
	if level < l.cfg.Level {
		return
	}

	now := time.Now()
	ti := traceFromCtx(ctx, l.cfg.ParentSpanIDs)

	var file string
	var lineno int
	if l.cfg.Caller {
		file, lineno = findCaller()
	}

	rec := Record{
		Timestamp:     now,
		Level:         level,
		Body:          msg,
		CodeFilepath:  file,
		CodeLineno:    lineno,
		TraceID:       ti.RootTraceID,
		SpanID:        ti.CurrentSpanID,
		ParentSpanIDs: ti.MiddleSpanIDs,
		Resource:      l.res,
		Attrs:         parseArgs(args),
	}

	// 结构化旁路：在序列化之前分发（未注册 Emitter 时无任何开销）
	if hs := l.hooks.Load(); hs != nil {
		for _, h := range *hs {
			emitSafe(ctx, h, &rec) // 捕获 Hook panic，防止击穿业务日志调用
		}
	}

	// 复用输出缓冲；锁外完成编码（格式化），锁内只做滚动判断与写入
	bp := bufPool.Get().(*[]byte)
	buf := l.enc.appendTo((*bp)[:0], &rec)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cfg.FileDir != "" {
		today := now.Format("2006-01-02")
		if today != l.currentDay {
			l.rotateFile(today)
		}
	}

	for _, w := range l.writers {
		if _, err := w.Write(buf); err != nil {
			l.reportError(err)
		}
	}

	// 归还缓冲
	*bp = buf
	bufPool.Put(bp)
}

// emitSafe 调用 Emitter 并捕获 panic（坏 Hook 不应影响业务日志调用）
func emitSafe(ctx context.Context, e Emitter, r *Record) {
	defer func() {
		if v := recover(); v != nil {
			fmt.Fprintf(os.Stderr, "logger: emitter panic: %v\n", v)
		}
	}()
	e.Emit(ctx, r)
}

// reportError 上报内部错误：优先调用 Config.OnError，否则写 stderr
func (l *Logger) reportError(err error) {
	if err == nil {
		return
	}
	if l.cfg.OnError != nil {
		l.cfg.OnError(err)
		return
	}
	fmt.Fprintf(os.Stderr, "logger: %v\n", err)
}

// Close 关闭日志文件
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// callerInfo 调用位置（文件与行号）
type callerInfo struct {
	file   string
	lineno int
}

// caller 收集缓存：避免每行日志重复做昂贵的栈解析
// 注：缓存无容量上限，规模受“不同日志调用点数量”约束（通常有限且稳定）
var (
	// callerCache 以首个业务帧的 pc 为 key，缓存其调用位置
	// 同一行代码重复打日志时直接命中，跳过栈解析
	callerCache sync.Map // map[uintptr]*callerInfo
	cwdOnce     sync.Once
	cwdValue    string
)

// callerCwd 获取工作目录（仅首次 syscall，后续命中缓存）
// 注：进程后续 chdir 不会刷新该缓存
func callerCwd() string {
	cwdOnce.Do(func() { cwdValue, _ = os.Getwd() })
	return cwdValue
}

// findCaller 向上找到第一个不属于 logger 包和运行时的调用位置
// 返回相对路径与行号（仅直接调用者，非调用栈）；同一位置二次调用命中缓存
func findCaller() (string, int) {
	// 栈数组而非堆分配；16 帧足够覆盖 logger 内部 2-3 帧 + 业务调用链
	var pcs [16]uintptr
	n := runtime.Callers(1, pcs[:])

	for i := 0; i < n; i++ {
		pc := pcs[i]
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			break
		}
		name := fn.Name()
		// 跳过 logger 包和 runtime 内部帧
		if strings.Contains(name, "github.com/memory198/go-gear/logger.") ||
			strings.HasPrefix(name, "runtime.") {
			continue
		}
		// 缓存命中：同一日志点直接复用
		if cached, ok := callerCache.Load(pc); ok {
			ci := cached.(*callerInfo)
			return ci.file, ci.lineno
		}
		file, line := fn.FileLine(pc)
		ci := &callerInfo{file: relativeFile(file), lineno: line}
		callerCache.Store(pc, ci)
		return ci.file, ci.lineno
	}
	return "", 0
}

// relativeFile 将绝对路径转为相对于当前工作目录的路径
// 不在工作目录下时，保留最后两级路径作为兜底
func relativeFile(absPath string) string {
	rel, err := filepath.Rel(callerCwd(), absPath)
	if err != nil {
		return shortFile(absPath)
	}
	return rel
}

// shortFile 只保留最后两段路径，如 service/user/user.go
func shortFile(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path
	}
	return strings.Join(parts[len(parts)-2:], "/")
}
