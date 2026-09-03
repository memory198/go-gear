package logger

import (
	"context"
	"sync/atomic"
)

// defaultLogger 包级默认日志实例，开箱即用
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

func Debug(ctx context.Context, msg string, args ...any)  { getDefault().Debug(ctx, msg, args...) }
func Info(ctx context.Context, msg string, args ...any)   { getDefault().Info(ctx, msg, args...) }
func Warn(ctx context.Context, msg string, args ...any)   { getDefault().Warn(ctx, msg, args...) }
func Error(ctx context.Context, msg string, args ...any)  { getDefault().Error(ctx, msg, args...) }
func Fatal(ctx context.Context, msg string, args ...any)  { getDefault().Fatal(ctx, msg, args...) }

func Debugf(ctx context.Context, format string, args ...any) { getDefault().Debugf(ctx, format, args...) }
func Infof(ctx context.Context, format string, args ...any)  { getDefault().Infof(ctx, format, args...) }
func Warnf(ctx context.Context, format string, args ...any)  { getDefault().Warnf(ctx, format, args...) }
func Errorf(ctx context.Context, format string, args ...any) { getDefault().Errorf(ctx, format, args...) }
func Fatalf(ctx context.Context, format string, args ...any) { getDefault().Fatalf(ctx, format, args...) }

// ---- 包级无 ctx 打印（log 风格） ----

func Print(msg string, args ...any)   { getDefault().Print(msg, args...) }
func Printf(format string, args ...any) { getDefault().Printf(format, args...) }

// Close 关闭包级默认日志打印器
func Close() error {
	return getDefault().Close()
}
