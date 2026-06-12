package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Config 日志配置
type Config struct {
	Level      Level
	Dir        string // 空则输出到 stdout
	Filename   string // 文件名（不含扩展名），空则取程序名
	RollingDay bool   // 按天滚动
}

// Logger 日志实例
type Logger struct {
	cfg        Config
	mu         sync.Mutex
	writer     io.Writer
	currentDay string
	file       *os.File
}

// New 创建 Logger
func New(cfg Config) (*Logger, error) {
	l := &Logger{cfg: cfg}
	if err := l.openWriter(); err != nil {
		return nil, err
	}
	return l, nil
}

// NewFromConfig 从配置参数创建
func NewFromConfig(level, dir, filename string, rollingDay bool) (*Logger, error) {
	return New(Config{
		Level:      parseLevel(level),
		Dir:        dir,
		Filename:   filename,
		RollingDay: rollingDay,
	})
}

// ---- 不带格式化 ----

func (l *Logger) Debug(ctx context.Context, msg string) { l.log(ctx, DEBUG, msg) }
func (l *Logger) Info(ctx context.Context, msg string)  { l.log(ctx, INFO, msg) }
func (l *Logger) Warn(ctx context.Context, msg string)  { l.log(ctx, WARN, msg) }
func (l *Logger) Error(ctx context.Context, msg string) { l.log(ctx, ERROR, msg) }

// ---- 带格式化 ----

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

// ErrorStack 打印 error 及其完整堆栈（使用 %+v）
func (l *Logger) ErrorStack(ctx context.Context, err error) {
	if err == nil {
		return
	}
	l.log(ctx, ERROR, fmt.Sprintf("%+v", err))
}

// log 核心写入逻辑
func (l *Logger) log(ctx context.Context, level Level, msg string) {
	if level < l.cfg.Level {
		return
	}

	now := time.Now()
	traceID := traceIDFromCtx(ctx)

	_, file, line, ok := runtime.Caller(2)
	caller := "???"
	if ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	var entry string
	if traceID != "" {
		entry = fmt.Sprintf("%s [%s] [%s] %s %s\n",
			now.Format("2006-01-02 15:04:05.000"),
			levelNames[level],
			traceID,
			caller,
			msg,
		)
	} else {
		entry = fmt.Sprintf("%s [%s] %s %s\n",
			now.Format("2006-01-02 15:04:05.000"),
			levelNames[level],
			caller,
			msg,
		)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cfg.RollingDay && l.cfg.Dir != "" {
		today := now.Format("2006-01-02")
		if today != l.currentDay {
			l.rotateFile(today)
		}
	}

	_, _ = fmt.Fprint(l.writer, entry)
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
