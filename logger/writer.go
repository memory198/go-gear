package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// openWriter 打开输出目标
func (l *Logger) openWriter() error {
	if l.cfg.Dir == "" {
		l.writer = os.Stdout
		return nil
	}

	if err := os.MkdirAll(l.cfg.Dir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	path := l.logFilePath(today)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	l.file = f
	l.writer = f
	l.currentDay = today
	return nil
}

// rotateFile 按天滚动到新的日志文件
func (l *Logger) rotateFile(today string) {
	if l.file != nil {
		_ = l.file.Close()
	}

	path := l.logFilePath(today)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: rotate failed: %v\n", err)
		return
	}

	l.file = f
	l.writer = f
	l.currentDay = today
}

// logFilePath 根据配置生成日志文件路径
func (l *Logger) logFilePath(day string) string {
	filename := l.cfg.Filename
	if filename == "" {
		filename = execName()
	}
	if l.cfg.RollingDay {
		return filepath.Join(l.cfg.Dir, fmt.Sprintf("%s.%s.log", filename, day))
	}
	return filepath.Join(l.cfg.Dir, fmt.Sprintf("%s.log", filename))
}

// execName 获取程序名（不含路径和扩展名）
func execName() string {
	exec, err := os.Executable()
	if err != nil {
		return "app"
	}
	name := filepath.Base(exec)
	if ext := filepath.Ext(name); ext != "" {
		name = name[:len(name)-len(ext)]
	}
	return name
}
