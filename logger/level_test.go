package logger

import "testing"

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Level
	}{
		{"trace lowercase", "trace", TRACE},
		{"debug lowercase", "debug", DEBUG},
		{"debug uppercase", "DEBUG", DEBUG},
		{"debug mixed case", "Debug", DEBUG},
		{"warn", "warn", WARN},
		{"error", "error", ERROR},
		{"fatal", "fatal", FATAL},
		{"info explicit", "info", INFO},
		{"empty defaults to info", "", INFO},
		{"unknown defaults to info", "verbose", INFO},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseLevel(tt.in); got != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
