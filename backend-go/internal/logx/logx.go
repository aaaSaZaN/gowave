// Package logx writes the service log: one line per event with a timestamp,
// level and component, readable in journalctl and `docker compose logs`.
// LOG_LEVEL (debug, info, warn, error) and LOG_FORMAT (text, json) tune it.
package logx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const componentKey = "component"

var level = new(slog.LevelVar)

func init() {
	// Until Setup runs (tests, early startup) lines still get the common format.
	slog.SetDefault(slog.New(NewTextHandler(os.Stdout, level)))
}

// ParseLevel accepts debug, info, warn (warning) and error; empty means info.
func ParseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return slog.LevelInfo, fmt.Errorf("unknown LOG_LEVEL %q: use debug, info, warn or error", value)
}

// Setup installs the process logger. The standard library's log package is
// routed through it too, so every line shares one format.
func Setup(levelName, format string) error {
	lvl, levelErr := ParseLevel(levelName)
	level.Set(lvl)
	var handler slog.Handler
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		handler = NewTextHandler(os.Stdout, level)
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	default:
		handler = NewTextHandler(os.Stdout, level)
		if levelErr == nil {
			levelErr = fmt.Errorf("unknown LOG_FORMAT %q: use text or json", format)
		}
	}
	slog.SetDefault(slog.New(handler))
	log.SetFlags(0)
	return levelErr
}

// Enabled reports whether lines of this level are written.
func Enabled(lvl slog.Level) bool { return level.Level() <= lvl }

func write(lvl slog.Level, component, msg string) {
	slog.Default().Log(context.Background(), lvl, msg, componentKey, component)
}

// The helpers format only when the level is written, so disabled debug
// lines cost nothing. They call fmt.Sprintf directly so go vet checks callers.
func Debugf(component, format string, args ...any) {
	if Enabled(slog.LevelDebug) {
		write(slog.LevelDebug, component, fmt.Sprintf(format, args...))
	}
}

func Infof(component, format string, args ...any) {
	if Enabled(slog.LevelInfo) {
		write(slog.LevelInfo, component, fmt.Sprintf(format, args...))
	}
}

func Warnf(component, format string, args ...any) {
	if Enabled(slog.LevelWarn) {
		write(slog.LevelWarn, component, fmt.Sprintf(format, args...))
	}
}

func Errorf(component, format string, args ...any) {
	if Enabled(slog.LevelError) {
		write(slog.LevelError, component, fmt.Sprintf(format, args...))
	}
}

// TextHandler prints `2006-01-02 15:04:05.000 INFO  [component] message key=value`.
type TextHandler struct {
	mu    *sync.Mutex
	w     io.Writer
	level slog.Leveler
	attrs []slog.Attr
}

func NewTextHandler(w io.Writer, lvl slog.Leveler) *TextHandler {
	return &TextHandler{mu: &sync.Mutex{}, w: w, level: lvl}
}

func (h *TextHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.level.Level()
}

func (h *TextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &clone
}

func (h *TextHandler) WithGroup(string) slog.Handler { return h }

func (h *TextHandler) Handle(_ context.Context, r slog.Record) error {
	var buf bytes.Buffer
	t := r.Time
	if t.IsZero() {
		t = time.Now()
	}
	buf.WriteString(t.UTC().Format("2006-01-02 15:04:05.000"))
	buf.WriteByte(' ')
	buf.WriteString(levelName(r.Level))
	component := ""
	var extra []slog.Attr
	collect := func(a slog.Attr) bool {
		if a.Key == componentKey {
			component = a.Value.String()
		} else if a.Key != "" {
			extra = append(extra, a)
		}
		return true
	}
	for _, a := range h.attrs {
		collect(a)
	}
	r.Attrs(collect)
	if component != "" {
		buf.WriteString(" [")
		buf.WriteString(component)
		buf.WriteByte(']')
	}
	buf.WriteByte(' ')
	buf.WriteString(strings.TrimRight(r.Message, "\n"))
	for _, a := range extra {
		buf.WriteByte(' ')
		buf.WriteString(a.Key)
		buf.WriteByte('=')
		value := a.Value.Resolve().String()
		if value == "" || strings.ContainsAny(value, " \t\"=") {
			value = strconv.Quote(value)
		}
		buf.WriteString(value)
	}
	buf.WriteByte('\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func levelName(lvl slog.Level) string {
	switch {
	case lvl >= slog.LevelError:
		return "ERROR"
	case lvl >= slog.LevelWarn:
		return "WARN "
	case lvl >= slog.LevelInfo:
		return "INFO "
	}
	return "DEBUG"
}
