package logger

import (
	"context"
	"io"
	"testing"
)

func benchLogger(format Format, caller bool) *Logger {
	cfg := Config{Level: DEBUG, Format: format, Caller: caller}
	enc := encoder(textEncoder{})
	if format == JSONFormat {
		enc = jsonEncoder{}
	}
	return &Logger{cfg: cfg, enc: enc, writers: []io.Writer{io.Discard}}
}

func BenchmarkText_NoCaller(b *testing.B) {
	l := benchLogger(TextFormat, false)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info(ctx, "user created", "user_id", 123, "source", "api")
	}
}

func BenchmarkText_Caller(b *testing.B) {
	l := benchLogger(TextFormat, true)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info(ctx, "user created", "user_id", 123, "source", "api")
	}
}

func BenchmarkJSON_NoCaller(b *testing.B) {
	l := benchLogger(JSONFormat, false)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info(ctx, "user created", "user_id", 123, "source", "api")
	}
}

func BenchmarkJSON_Caller(b *testing.B) {
	l := benchLogger(JSONFormat, true)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info(ctx, "user created", "user_id", 123, "source", "api")
	}
}
