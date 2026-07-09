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
	Level    Level  // 最低输出等级
	Format   Format // TextFormat（默认）/ JSONFormat
	Console  bool   // 是否输出到控制台（和文件可同时）
	FileDir  string // 文件输出目录，空则不写文件
	Filename string // 文件名（不含扩展名），空则取程序名
	MaxAge   int    // 日志保留天数，<=0 不清理
	Caller   bool   // 是否输出调用文件和行号，默认 true
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

func (l *Logger) Trace(ctx context.Context, msg string) { l.log(ctx, TRACE, msg) }
func (l *Logger) Debug(ctx context.Context, msg string)  { l.log(ctx, DEBUG, msg) }
func (l *Logger) Info(ctx context.Context, msg string)   { l.log(ctx, INFO, msg) }
func (l *Logger) Warn(ctx context.Context, msg string)   { l.log(ctx, WARN, msg) }
func (l *Logger) Error(ctx context.Context, msg string)  { l.log(ctx, ERROR, msg) }

// Fatal 输出 FATAL 等级日志后退出程序（调用 os.Exit(1)，defer 不会执行）
func (l *Logger) Fatal(ctx context.Context, msg string) {
	l.log(ctx, FATAL, msg)
	os.Exit(1)
}

// ---- 实例方法：带格式化 ----

func (l *Logger) Tracef(ctx context.Context, format string, args ...any) {
	l.log(ctx, TRACE, fmt.Sprintf(format, args...))
}
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

// log 核心写入逻辑
func (l *Logger) log(ctx context.Context, level Level, msg string) {
	if level < l.cfg.Level {
		return
	}

	now := time.Now()
	ti := traceFromCtx(ctx)

	var caller string
	if l.cfg.Caller {
		caller = findCaller()
	}

	e := &entry{
		Time:          now.Format("2006-01-02 15:04:05.000000"),
		Level:         levelNames[level],
		Msg:           msg,
		Caller:        caller,
		RootTraceID:   ti.RootID,
		MiddleSpanIDs: ti.MiddleIDs,
		CurrentSpanID: ti.CurrentID,
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

// findCaller 向上遍历调用栈，找到第一个不属于 logger 包和运行时的帧
// 返回 "相对路径:行号"，不受编译器内联影响
func findCaller() string {
	const maxDepth = 15
	pcs := make([]uintptr, maxDepth)
	// skip=1 跳过 runtime.Callers 自身
	n := runtime.Callers(1, pcs)
	frames := runtime.CallersFrames(pcs[:n])

	for {
		f, more := frames.Next()
		if f.Function == "" {
			break
		}
		// 跳过 logger 包和 runtime 内部帧
		if strings.Contains(f.Function, "github.com/memory198/go-gear/logger.") ||
			strings.HasPrefix(f.Function, "runtime.") {
			if !more {
				break
			}
			continue
		}
		return fmt.Sprintf("%s:%d", relativeFile(f.File), f.Line)
	}
	return "???"
}

// relativeFile 将绝对路径转为相对于当前工作目录的路径
// 不在工作目录下时，保留最后两级路径作为兜底
func relativeFile(absPath string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return shortFile(absPath)
	}
	rel, err := filepath.Rel(cwd, absPath)
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
