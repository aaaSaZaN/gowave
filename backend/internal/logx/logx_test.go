package logx

import (
	"bytes"
	"log/slog"
	"regexp"
	"testing"
)

func TestTextHandlerFormat(t *testing.T) {
	var buf bytes.Buffer
	lvl := new(slog.LevelVar)
	logger := slog.New(NewTextHandler(&buf, lvl))

	logger.Info("Node de-1 started", componentKey, "nodes", "took", "12ms", "reason", "two words")
	logger.Debug("hidden", componentKey, "nodes")
	logger.Warn("no component")

	want := regexp.MustCompile(`^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{3} INFO  \[nodes\] Node de-1 started took=12ms reason="two words"\n` +
		`\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{3} WARN  no component\n$`)
	if !want.Match(buf.Bytes()) {
		t.Fatalf("unexpected output:\n%s", buf.String())
	}
}

func TestParseLevel(t *testing.T) {
	for input, want := range map[string]slog.Level{"": slog.LevelInfo, "DEBUG": slog.LevelDebug, "warning": slog.LevelWarn, "error": slog.LevelError} {
		if got, err := ParseLevel(input); err != nil || got != want {
			t.Errorf("ParseLevel(%q) = %v, %v; want %v", input, got, err, want)
		}
	}
	if _, err := ParseLevel("verbose"); err == nil {
		t.Error("ParseLevel accepted an unknown level")
	}
}

func TestLevelFiltersHelpers(t *testing.T) {
	old := level.Level()
	t.Cleanup(func() { level.Set(old) })
	level.Set(slog.LevelWarn)
	if Enabled(slog.LevelInfo) || !Enabled(slog.LevelError) {
		t.Fatal("LOG_LEVEL=warn must hide info and keep errors")
	}
}
