package logger_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/memory198/go-gear/logger"
)

// caller 测试放在外部测试包（logger_test）：
// findCaller 会跳过 logger 包自身的调用帧，包内测试函数名带 logger. 前缀会被误跳过，
// 因此必须从包外发起调用才能验证真实的调用位置。

//go:noinline
func callerLevelOne(l *logger.Logger, msg string) {
	l.Info(context.Background(), msg)
}

// readLog 读取 Logger 当天写出的日志文件内容
func readLog(t *testing.T, dir string) string {
	t.Helper()
	day := time.Now().Format("2006-01-02")
	data, err := os.ReadFile(filepath.Join(dir, "test."+day+".log"))
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	return string(data)
}

func TestCallerEnabled(t *testing.T) {
	dir := t.TempDir()
	l, err := logger.New(logger.Config{Level: logger.DEBUG, FileDir: dir, Filename: "test", Caller: true})
	if err != nil {
		t.Fatal(err)
	}
	callerLevelOne(l, "with caller")
	l.Close()

	out := readLog(t, dir)
	// 应输出一行直接调用位置（本文件内），且不包含多层分隔符 ->（非调用栈）
	if !strings.Contains(out, "caller_test.go:") {
		t.Errorf("Caller=true should contain one caller line: %q", out)
	}
	if strings.Contains(out, "->") {
		t.Errorf("Caller should be single line, not a call stack: %q", out)
	}
}

func TestCallerDisabled(t *testing.T) {
	dir := t.TempDir()
	l, err := logger.New(logger.Config{Level: logger.DEBUG, FileDir: dir, Filename: "test", Caller: false})
	if err != nil {
		t.Fatal(err)
	}
	callerLevelOne(l, "no caller")
	l.Close()

	out := readLog(t, dir)
	if strings.Contains(out, ".go:") {
		t.Errorf("Caller=false should not contain caller position: %q", out)
	}
}
