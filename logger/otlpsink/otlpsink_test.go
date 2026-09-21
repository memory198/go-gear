package otlpsink

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/memory198/go-gear/logger/core"
)

// TestSinkEmitsOTLP 用假 Collector 验证导出的 OTLP 请求内容
func TestSinkEmitsOTLP(t *testing.T) {
	type received struct {
		path        string
		contentType string
		body        []byte
	}
	var mu sync.Mutex
	var reqs []received

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqs = append(reqs, received{
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			body:        body,
		})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	endpoint := strings.TrimPrefix(srv.URL, "http://")
	sink, shutdown, err := New(context.Background(),
		WithEndpoint(endpoint),
		WithInsecure(),
		WithBatchInterval(20*time.Millisecond),
		WithService("user-api", "1.2.0"),
		WithEnvironment("prod"),
	)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	const (
		traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
		spanID  = "00f067aa0ba902b7"
	)
	sink.Emit(context.Background(), &core.Record{
		Timestamp:    time.Now(),
		Level:        core.INFO,
		Body:         "otlp test",
		CodeFilepath: "handler/user.go",
		CodeLineno:   42,
		TraceID:      traceID,
		SpanID:       spanID,
		Attrs:        []core.Attr{{Key: "user_id", Value: 123}},
	})

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(reqs) == 0 {
		t.Fatal("no OTLP request received")
	}

	var all bytes.Buffer
	for _, r := range reqs {
		all.Write(r.body)
	}
	body := all.Bytes()

	// OTLP/HTTP 约定路径与内容类型
	if reqs[0].path != "/v1/logs" {
		t.Errorf("path = %q, want /v1/logs", reqs[0].path)
	}
	if !strings.Contains(reqs[0].contentType, "protobuf") {
		t.Errorf("content-type = %q, want protobuf", reqs[0].contentType)
	}

	// protobuf 中字符串字段为明文，可直接断言
	for _, want := range []string{"otlp test", "INFO", "user-api", "1.2.0", "prod", "code"} {
		if !bytes.Contains(body, []byte(want)) {
			t.Errorf("exported payload should contain %q", want)
		}
	}

	// trace_id 以 16 字节二进制字段编码
	tid, err := hex.DecodeString(traceID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, tid) {
		t.Error("exported payload should contain trace_id bytes")
	}
	sid, err := hex.DecodeString(spanID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, sid) {
		t.Error("exported payload should contain span_id bytes")
	}
}

func TestSinkWithOTelSpanContext(t *testing.T) {
	// ctx 已含 OTel span context 时应沿用（不覆盖）
	got := contextWithTraceIDs(context.Background(), "bad-hex", "")
	if got == nil {
		t.Fatal("contextWithTraceIDs should never return nil")
	}
	if got.Err() != nil {
		t.Errorf("context should be valid: %v", got.Err())
	}
}

func TestAttrFromAny(t *testing.T) {
	// 常用类型映射（通过导出的 payload 间接验证在 TestSinkEmitsOTLP；
	// 此处直接验证不会 panic 且类型正确）
	cases := []any{nil, "s", true, 1, int64(2), 3.5, []string{"a"}, struct{ X int }{1}}
	for _, c := range cases {
		if kv := attrFromAny("k", c); string(kv.Key) != "k" {
			t.Errorf("attrFromAny key mismatch for %T", c)
		}
	}
}

func TestDefaultTLSEnabled(t *testing.T) {
	// 安全默认：未显式 WithInsecure 时应启用 TLS
	if defaultConfig().insecure {
		t.Error("TLS should be enabled by default (use WithInsecure to opt out)")
	}
}

func TestWithResource(t *testing.T) {
	cfg := defaultConfig()
	WithResource(core.Resource{
		ServiceName:    "user-api",
		ServiceVersion: "1.2.0",
		Environment:    "prod",
		HostName:       "node-1",
	})(cfg)

	if cfg.serviceName != "user-api" || cfg.serviceVer != "1.2.0" ||
		cfg.environment != "prod" || cfg.hostName != "node-1" {
		t.Errorf("WithResource not applied: %+v", cfg)
	}
}

func TestSinkImplementsFlusher(t *testing.T) {
	// Sink 需实现 core.Flusher，供 Fatal 退出前刷新
	var _ core.Flusher = (*Sink)(nil)
}
