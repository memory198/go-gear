package core

import "testing"

func TestLevelValuesAlignOTel(t *testing.T) {
	// 级别数值对齐 OTel SeverityNumber 代表值
	tests := []struct {
		level Level
		want  int
	}{
		{DEBUG, 5},
		{INFO, 9},
		{WARN, 13},
		{ERROR, 17},
		{FATAL, 21},
	}
	for _, tt := range tests {
		if int(tt.level) != tt.want {
			t.Errorf("%s = %d, want %d (OTel SeverityNumber)", tt.level, tt.level, tt.want)
		}
	}
}

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{FATAL, "FATAL"},
		{Level(99), "LEVEL(99)"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in   string
		want Level
	}{
		{"debug", DEBUG},
		{"DEBUG", DEBUG},
		{"warn", WARN},
		{"error", ERROR},
		{"fatal", FATAL},
		{"info", INFO},
		{"", INFO},
		{"verbose", INFO},
	}
	for _, tt := range tests {
		if got := ParseLevel(tt.in); got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestResourceEmpty(t *testing.T) {
	if !(Resource{}).Empty() {
		t.Error("zero Resource should be Empty")
	}
	if (Resource{ServiceName: "x"}).Empty() {
		t.Error("Resource with service name should not be Empty")
	}
}
