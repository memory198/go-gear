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

func Debug(ctx context.Context, msg string)  { getDefault().Debug(ctx, msg) }
func Info(ctx context.Context, msg string)   { getDefault().Info(ctx, msg) }
func Warn(ctx context.Context, msg string)   { getDefault().Warn(ctx, msg) }
func Error(ctx context.Context, msg string)  { getDefault().Error(ctx, msg) }
func Fatal(ctx context.Context, msg string)  { getDefault().Fatal(ctx, msg) }

func Debugf(ctx context.Context, format string, args ...any) { getDefault().Debugf(ctx, format, args...) }
func Infof(ctx context.Context, format string, args ...any)  { getDefault().Infof(ctx, format, args...) }
func Warnf(ctx context.Context, format string, args ...any)  { getDefault().Warnf(ctx, format, args...) }
func Errorf(ctx context.Context, format string, args ...any) { getDefault().Errorf(ctx, format, args...) }
func Fatalf(ctx context.Context, format string, args ...any) { getDefault().Fatalf(ctx, format, args...) }

// Close 关闭包级默认日志打印器
func Close() error {
	return getDefault().Close()
}
