package logger

import (
	"context"
	"sync/atomic"
)

// defaultLogger 包级默认日志实例，开箱即用
//
// 使用 atomic.Pointer 而非普通字段的原因：
//   - 读：包级快捷方法（Info/Print 等）在任意 goroutine 高频调用 getDefault()，
//     日志是最热路径，要求无锁读（Load 为原子指令，零竞争开销）
//   - 写：SetDefault 允许运行时替换默认实例（如 main 按配置初始化后替换），
//     与并发读构成读写竞争，需原子同步保证 happens-before
//   - 实例内部的并发写入由 Logger 自身的 mutex 保护，与本处原子读写分层解决
var defaultLogger atomic.Pointer[Logger]

func init() {
	l, _ := New(Config{
		Level:   DEBUG,
		Format:  TextFormat,
		Console: true,
		Caller:  true,
	})
	defaultLogger.Store(l)
}

// SetDefault 替换包级默认日志打印器（线程安全）
func SetDefault(l *Logger) {
	defaultLogger.Store(l)
}

func getDefault() *Logger {
	return defaultLogger.Load()
}

// ---- 包级快捷方法 ----

func Debug(ctx context.Context, msg string, args ...any) { getDefault().Debug(ctx, msg, args...) }
func Info(ctx context.Context, msg string, args ...any)  { getDefault().Info(ctx, msg, args...) }
func Warn(ctx context.Context, msg string, args ...any)  { getDefault().Warn(ctx, msg, args...) }
func Error(ctx context.Context, msg string, args ...any) { getDefault().Error(ctx, msg, args...) }
func Fatal(ctx context.Context, msg string, args ...any) { getDefault().Fatal(ctx, msg, args...) }

func Debugf(ctx context.Context, format string, args ...any) {
	getDefault().Debugf(ctx, format, args...)
}
func Infof(ctx context.Context, format string, args ...any) { getDefault().Infof(ctx, format, args...) }
func Warnf(ctx context.Context, format string, args ...any) { getDefault().Warnf(ctx, format, args...) }
func Errorf(ctx context.Context, format string, args ...any) {
	getDefault().Errorf(ctx, format, args...)
}
func Fatalf(ctx context.Context, format string, args ...any) {
	getDefault().Fatalf(ctx, format, args...)
}

// ---- 包级无 ctx 打印（log 风格） ----

func Print(msg string, args ...any)     { getDefault().Print(msg, args...) }
func Printf(format string, args ...any) { getDefault().Printf(format, args...) }

// Close 关闭包级默认日志打印器
func Close() error {
	return getDefault().Close()
}
