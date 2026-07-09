package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// openWriters 根据配置打开所有输出目标
// console 和 file 可同时存在
func (l *Logger) openWriters() error {
	if l.cfg.Console || l.cfg.FileDir == "" {
		l.writers = append(l.writers, os.Stdout)
	}

	if l.cfg.FileDir != "" {
		if err := l.openFile(); err != nil {
			return err
		}
		l.writers = append(l.writers, l.file)
		// 启动时清理过期日志
		l.cleanExpiredLogs()
	}

	return nil
}

// openFile 打开日志文件，每日滚动
func (l *Logger) openFile() error {
	if err := os.MkdirAll(l.cfg.FileDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	path := l.logFilePath(today)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	l.file = f
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

	// 将 writers 中的旧文件引用替换为新文件
	for i, w := range l.writers {
		if w == l.file {
			l.writers[i] = f
			break
		}
	}

	l.file = f
	l.currentDay = today

	// 滚动后清理过期日志
	l.cleanExpiredLogs()
}

// cleanExpiredLogs 清理超过 MaxAge 天的日志文件
func (l *Logger) cleanExpiredLogs() {
	if l.cfg.MaxAge <= 0 {
		return
	}

	entries, err := os.ReadDir(l.cfg.FileDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: cleanup read dir failed: %v\n", err)
		return
	}

	// 构造文件名前缀，用于匹配属于本 Logger 的日志文件
	prefix := l.logFilePrefix()

	cutoff := time.Now().AddDate(0, 0, -l.cfg.MaxAge)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 只清理匹配前缀且后缀为 .log 或 .日期.log 的文件
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(l.cfg.FileDir, name)
			if err := os.Remove(path); err != nil {
				fmt.Fprintf(os.Stderr, "logger: cleanup remove %s failed: %v\n", path, err)
			}
		}
	}
}

// logFilePath 根据配置生成日志文件路径
func (l *Logger) logFilePath(day string) string {
	prefix := l.logFilePrefix()
	return filepath.Join(l.cfg.FileDir, fmt.Sprintf("%s.%s.log", prefix, day))
}

// logFilePrefix 日志文件名前缀（不含日期和扩展名）
func (l *Logger) logFilePrefix() string {
	filename := l.cfg.Filename
	if filename == "" {
		filename = execName()
	}
	return filename
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

// 确保编译时检查不必要的导入
var _ io.Writer = os.Stdout
