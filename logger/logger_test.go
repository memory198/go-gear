package logger

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// newTestLogger 构造写入内存 buffer 的 Logger，绕过文件系统便于断言
func newTestLogger(level Level) (*Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return &Logger{
		cfg:     Config{Level: level, Caller: true},
		enc:     textEncoder{},
		writers: []io.Writer{buf},
	}, buf
}

func TestLevelFiltering(t *testing.T) {
	tests := []struct {
		name       string
		cfgLevel   Level
		call       func(l *Logger, ctx context.Context)
		wantOutput bool
	}{
		{"debug filtered at info level", INFO, func(l *Logger, ctx context.Context) { l.Debug(ctx, "x") }, false},
		{"info passes at info level", INFO, func(l *Logger, ctx context.Context) { l.Info(ctx, "x") }, true},
		{"warn filtered at error level", ERROR, func(l *Logger, ctx context.Context) { l.Warn(ctx, "x") }, false},
		{"error passes at error level", ERROR, func(l *Logger, ctx context.Context) { l.Error(ctx, "x") }, true},
		{"debug passes at debug level", DEBUG, func(l *Logger, ctx context.Context) { l.Debug(ctx, "x") }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, buf := newTestLogger(tt.cfgLevel)
			tt.call(l, context.Background())
			got := buf.Len() > 0
			if got != tt.wantOutput {
				t.Errorf("output produced = %v, want %v (buf=%q)", got, tt.wantOutput, buf.String())
			}
		})
	}
}

func TestLogFormat_NoTraceIDs(t *testing.T) {
	l, buf := newTestLogger(DEBUG)
	l.Info(context.Background(), "hello world")

	out := buf.String()
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}[+-]\d{2}:\d{2} \[INFO\] .+:\d+ hello world\n$`)
	if !re.MatchString(out) {
		t.Errorf("output format mismatch: %q", out)
	}
}

func TestLogFormat_WithRootTraceID(t *testing.T) {
	l, buf := newTestLogger(DEBUG)
	ctx := context.WithValue(context.Background(), RootTraceIDKey, "root-1")
	l.Info(ctx, "hello")

	out := buf.String()
	// text 格式仅展示 trace_id，caller 用中括号包裹
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}[+-]\d{2}:\d{2} \[INFO\] \[root-1\] .+:\d+ hello\n$`)
	if !re.MatchString(out) {
		t.Errorf("output format mismatch: %q", out)
	}
}

func TestLogJSONFormat_AllTraceFields(t *testing.T) {
	buf := &bytes.Buffer{}
	l := &Logger{
		cfg:     Config{Level: DEBUG, Format: JSONFormat, Caller: true, ParentSpanIDs: true},
		enc:     jsonEncoder{},
		writers: []io.Writer{buf},
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, RootTraceIDKey, "r")
	ctx = context.WithValue(ctx, MiddleSpanIDsKey, []string{"m1", "m2"})
	ctx = context.WithValue(ctx, CurrentSpanIDKey, "c")

	l.Info(ctx, "json test")

	out := buf.String()
	if !strings.Contains(out, `"trace_id":"r"`) {
		t.Errorf("expected trace_id in JSON, got %q", out)
	}
	if !strings.Contains(out, `"parent_span_ids":["m1","m2"]`) {
		t.Errorf("expected parent_span_ids array in JSON, got %q", out)
	}
	if !strings.Contains(out, `"span_id":"c"`) {
		t.Errorf("expected span_id in JSON, got %q", out)
	}
	if !strings.Contains(out, `"body":"json test"`) {
		t.Errorf("expected body in JSON, got %q", out)
	}
	if !strings.Contains(out, `"severity_text":"INFO"`) {
		t.Errorf("expected severity_text in JSON, got %q", out)
	}
	if !strings.Contains(out, `"code.filepath"`) || !strings.Contains(out, `"code.lineno"`) {
		t.Errorf("expected code.filepath/code.lineno in JSON, got %q", out)
	}
}

func TestLogJSONFormat_MiddleSpanIDsDisabledByDefault(t *testing.T) {
	// 默认（未开启 MiddleSpanIDs）不应输出 parent_span_ids
	buf := &bytes.Buffer{}
	l := &Logger{
		cfg:     Config{Level: DEBUG, Format: JSONFormat},
		enc:     jsonEncoder{},
		writers: []io.Writer{buf},
	}

	ctx := context.Background()
	ctx = context.WithValue(ctx, RootTraceIDKey, "r")
	ctx = context.WithValue(ctx, MiddleSpanIDsKey, []string{"m1", "m2"})

	l.Info(ctx, "no middle span ids")

	out := buf.String()
	if strings.Contains(out, "parent_span_ids") {
		t.Errorf("parent_span_ids should be omitted by default: %q", out)
	}
	if !strings.Contains(out, `"trace_id":"r"`) {
		t.Errorf("trace_id should still be present: %q", out)
	}
}

func TestLogJSONFormat_NoTraceFields(t *testing.T) {
	buf := &bytes.Buffer{}
	l := &Logger{
		cfg:     Config{Level: DEBUG, Format: JSONFormat, Caller: true},
		enc:     jsonEncoder{},
		writers: []io.Writer{buf},
	}
	l.Info(context.Background(), "no trace")
	out := buf.String()
	// 无 trace 时不输出对应字段
	if strings.Contains(out, "trace_id") || strings.Contains(out, "span_id") {
		t.Errorf("unexpected trace fields in JSON when not set: %q", out)
	}
}

func TestFormattedMethods(t *testing.T) {
	tests := []struct {
		name string
		call func(l *Logger, ctx context.Context)
		want string
	}{
		{"Debugf", func(l *Logger, ctx context.Context) { l.Debugf(ctx, "user %s id=%d", "alice", 1) }, "user alice id=1"},
		{"Infof", func(l *Logger, ctx context.Context) { l.Infof(ctx, "n=%d", 5) }, "n=5"},
		{"Warnf", func(l *Logger, ctx context.Context) { l.Warnf(ctx, "warn %s", "x") }, "warn x"},
		{"Errorf", func(l *Logger, ctx context.Context) { l.Errorf(ctx, "err %s", "y") }, "err y"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, buf := newTestLogger(DEBUG)
			tt.call(l, context.Background())
			if !strings.Contains(buf.String(), tt.want) {
				t.Errorf("output = %q, want contains %q", buf.String(), tt.want)
			}
		})
	}
}

func TestTextFormatWithFields(t *testing.T) {
	l, buf := newTestLogger(DEBUG)
	l.Info(context.Background(), "user created", "user_id", 123, "source", "api")

	out := buf.String()
	if !strings.Contains(out, "user created user_id=123 source=api") {
		t.Errorf("text format should append k=v fields: %q", out)
	}
}

func TestJSONFormatWithFields(t *testing.T) {
	buf := &bytes.Buffer{}
	l := &Logger{
		cfg:     Config{Level: DEBUG, Format: JSONFormat, Caller: false},
		enc:     jsonEncoder{},
		writers: []io.Writer{buf},
	}
	l.Info(context.Background(), "user created", "user_id", 123, "source", "api")

	out := buf.String()
	if !strings.Contains(out, `"user_id":123`) {
		t.Errorf("JSON should include kv field user_id: %q", out)
	}
	if !strings.Contains(out, `"source":"api"`) {
		t.Errorf("JSON should include kv field source: %q", out)
	}
	if !strings.Contains(out, `"body":"user created"`) {
		t.Errorf("JSON should keep body intact: %q", out)
	}
	// kv 归入 attributes 对象
	if !strings.Contains(out, `"attributes":{`) {
		t.Errorf("kv should be nested under attributes: %q", out)
	}
}

func TestJSONFormatWithFieldsReservedKeyIgnored(t *testing.T) {
	// kv key 与内置字段同名（如 severity_text）应被忽略，不破坏结构
	buf := &bytes.Buffer{}
	l := &Logger{
		cfg:     Config{Level: DEBUG, Format: JSONFormat, Caller: false},
		enc:     jsonEncoder{},
		writers: []io.Writer{buf},
	}
	l.Info(context.Background(), "msg", "severity_text", "hacked", "body", "hacked2")

	out := buf.String()
	if strings.Contains(out, "hacked") {
		t.Errorf("kv should not override reserved fields: %q", out)
	}
}

func TestJSONFormatWithResource(t *testing.T) {
	// resource 元信息嵌套输出（点分键）
	buf := &bytes.Buffer{}
	l := &Logger{
		cfg: Config{Level: DEBUG, Format: JSONFormat},
		enc: jsonEncoder{},
		res: resource{
			ServiceName:    "user-api",
			ServiceVersion: "1.2.0",
			Environment:    "prod",
			HostName:       "node-1",
		},
		writers: []io.Writer{buf},
	}
	l.Info(context.Background(), "with resource")

	out := buf.String()
	for _, want := range []string{
		`"resource":{`, `"service.name":"user-api"`, `"service.version":"1.2.0"`,
		`"deployment.environment":"prod"`, `"host.name":"node-1"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("resource output missing %s: %q", want, out)
		}
	}
}

func TestJSONFormatEscape(t *testing.T) {
	// 引号/反斜杠/换行等需要正确转义
	buf := &bytes.Buffer{}
	l := &Logger{
		cfg:     Config{Level: DEBUG, Format: JSONFormat},
		enc:     jsonEncoder{},
		writers: []io.Writer{buf},
	}
	l.Info(context.Background(), "say \"hi\"\nline2", "path", `C:\tmp\a`)

	out := buf.String()
	if !strings.Contains(out, `"body":"say \"hi\"\nline2"`) {
		t.Errorf("body escaping failed: %q", out)
	}
	if !strings.Contains(out, `"path":"C:\\tmp\\a"`) {
		t.Errorf("attribute escaping failed: %q", out)
	}
}

func TestOddArgsBadKey(t *testing.T) {
	// 奇数个 kv 参数：末尾裸值记为 !BADKEY（slog 约定）
	l, buf := newTestLogger(DEBUG)
	l.Info(context.Background(), "odd", "user_id", 123, "orphan")

	out := buf.String()
	if !strings.Contains(out, "user_id=123 !BADKEY=orphan") {
		t.Errorf("odd args tail should be !BADKEY: %q", out)
	}
}

func TestPrintNoTrace(t *testing.T) {
	// Print 为 INFO 级且不携带 trace 字段
	l, buf := newTestLogger(DEBUG)
	l.Print("server starting", "port", 8080)

	out := buf.String()
	if !strings.Contains(out, "[INFO]") {
		t.Errorf("Print should be INFO level: %q", out)
	}
	if strings.Contains(out, "[root-") {
		t.Errorf("Print should not carry trace fields: %q", out)
	}
	if !strings.Contains(out, "server starting port=8080") {
		t.Errorf("Print should append kv fields: %q", out)
	}
}

func TestPrintfFormatting(t *testing.T) {
	l, buf := newTestLogger(DEBUG)
	l.Printf("server on %s:%d", "0.0.0.0", 8080)

	out := buf.String()
	if !strings.Contains(out, "server on 0.0.0.0:8080") {
		t.Errorf("Printf should format message: %q", out)
	}
}

func TestPrintLevelFiltered(t *testing.T) {
	// Print 是 INFO 级：INFO 级别下应输出，ERROR 级别下应被过滤
	l, buf := newTestLogger(ERROR)
	l.Print("should be filtered")
	if buf.Len() > 0 {
		t.Errorf("Print(INFO) should be filtered at ERROR level: %q", buf.String())
	}
}

func TestPackageLevelPrint(t *testing.T) {
	buf := &bytes.Buffer{}
	l := &Logger{cfg: Config{Level: DEBUG}, enc: textEncoder{}, writers: []io.Writer{buf}}
	old := getDefault()
	SetDefault(l)
	defer SetDefault(old)

	Print("pkg print")
	Printf("pkg printf %d", 1)
	out := buf.String()
	if !strings.Contains(out, "pkg print") || !strings.Contains(out, "pkg printf 1") {
		t.Errorf("package-level Print/Printf should route to default logger: %q", out)
	}
}

func TestConcurrentWrites(t *testing.T) {
	l, buf := newTestLogger(DEBUG)
	const n = 50

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Info(context.Background(), "concurrent")
		}()
	}
	wg.Wait()

	lines := strings.Count(buf.String(), "\n")
	if lines != n {
		t.Errorf("expected %d log lines, got %d (mutex should prevent interleaving)", n, lines)
	}
}

func TestNoCaller(t *testing.T) {
	buf := &bytes.Buffer{}
	l := &Logger{
		cfg:     Config{Level: DEBUG, Caller: false},
		enc:     textEncoder{},
		writers: []io.Writer{buf},
	}
	l.Info(context.Background(), "no caller")
	out := buf.String()
	// Caller=false 时输出中不应出现 .go:行号
	if strings.Contains(out, ".go:") {
		t.Errorf("expected no file:line when Caller=false, got %q", out)
	}
}

func TestFatal(t *testing.T) {
	// Fatal 会 os.Exit(1)，需要子进程测试
	// 此处仅验证 FATAL 级别存在且日志输出级别不低于 FATAL 的级别都能正常写入
	l, buf := newTestLogger(INFO)
	// FATAL 级别 >= INFO，应正常输出
	l.log(context.Background(), FATAL, "fatal message")
	out := buf.String()
	if !strings.Contains(out, "[FATAL]") {
		t.Errorf("FATAL log should be written: %q", out)
	}
	if !strings.Contains(out, "fatal message") {
		t.Errorf("FATAL log should contain message: %q", out)
	}
}
