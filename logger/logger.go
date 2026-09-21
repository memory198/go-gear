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
	"time"
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
	MiddleSpanIDs bool   // 是否输出中间 span ID 链（默认关闭；开启需读取 gctx 聚合，有开销）
}

// Logger 日志实例
type Logger struct {
	cfg        Config      // 日志配置（级别、格式、输出渠道等）
	mu         sync.Mutex  // 并发写入锁，保证日志行不交叉
	enc        encoder     // 日志编码器（text 或 json）
	writers    []io.Writer // 输出目标列表（stdout + 文件可同时存在）
	currentDay string      // 当前日志文件所属日期，用于判断是否需要滚动
	file       *os.File    // 当前打开的日志文件句柄，仅文件输出时非 nil
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

	if err := l.openWriters(); err != nil {
		return nil, err
	}
	return l, nil
}

// NewFromConfig 从配置参数创建
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
func (l *Logger) Fatal(ctx context.Context, msg string, args ...any) {
	l.log(ctx, FATAL, msg, args...)
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
func (l *Logger) Fatalf(ctx context.Context, format string, args ...any) {
	l.log(ctx, FATAL, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// ---- 无 ctx 打印（log 风格） ----
// 面向启动、后台任务等非请求场景：内部使用 context.Background()，
// 因此不携带 trace 链路字段；请求内日志请使用 Info(ctx, ...) 等带 ctx 方法。

// Print 输出 INFO 级日志，args 为 slog 键值对字段
func (l *Logger) Print(msg string, args ...any) {
	l.log(context.Background(), INFO, msg, args...)
}

// Printf 格式化输出 INFO 级日志（无 ctx）
func (l *Logger) Printf(format string, args ...any) {
	l.log(context.Background(), INFO, fmt.Sprintf(format, args...))
}

// log 核心写入逻辑
func (l *Logger) log(ctx context.Context, level Level, msg string, args ...any) {
	if level < l.cfg.Level {
		return
	}

	now := time.Now()
	ti := traceFromCtx(ctx, l.cfg.MiddleSpanIDs)

	var caller string
	if l.cfg.Caller {
		caller = findCaller()
	}

	e := &entry{
		Time:          now.Format("2006-01-02 15:04:05.000000"),
		Level:         levelNames[level],
		Msg:           msg,
		Caller:        caller,
		RootTraceID:   ti.RootTraceID,
		MiddleSpanIDs: ti.MiddleSpanIDs,
		CurrentSpanID: ti.CurrentSpanID,
		fields:        parseArgs(args),
	}

	output := l.enc.encode(e)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cfg.FileDir != "" {
		today := now.Format("2006-01-02")
		if today != l.currentDay {
			l.rotateFile(today)
		}
	}

	for _, w := range l.writers {
		_, _ = fmt.Fprint(w, output)
	}
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

// caller 收集缓存：避免每行日志重复做昂贵的栈解析
var (
	// callerCache 以首个业务帧的 pc 为 key，缓存其 "file:line"
	// 同一行代码重复打日志时直接命中，跳过栈解析
	callerCache sync.Map // map[uintptr]string
	cwdOnce     sync.Once
	cwdValue    string
)

// callerCwd 获取工作目录（仅首次 syscall，后续命中缓存）
func callerCwd() string {
	cwdOnce.Do(func() { cwdValue, _ = os.Getwd() })
	return cwdValue
}

// findCaller 向上找到第一个不属于 logger 包和运行时的调用位置，返回 "相对路径:行号"
// 仅取一行（直接调用者），不打印调用栈；同一位置二次调用命中缓存
func findCaller() string {
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
			return cached.(string)
		}
		file, line := fn.FileLine(pc)
		pos := fmt.Sprintf("%s:%d", relativeFile(file), line)
		callerCache.LoadOrStore(pc, pos)
		return pos
	}
	return ""
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
