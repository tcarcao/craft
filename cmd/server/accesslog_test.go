package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureLogs points the default logger at a JSON buffer at level for the test.
func captureLogs(t *testing.T, level string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(newLogger(&buf, "json", level))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("log line is not JSON: %q", l)
		}
		lines = append(lines, m)
	}
	return lines
}

func serveVia(t *testing.T, method, path, body string) {
	t.Helper()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	newHandler(server, "/craft").ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, path, strings.NewReader(body)))
}

// Probes call /health every few seconds; at the default level they log nothing.
func TestAccessLog_HealthOnlyAtDebug(t *testing.T) {
	buf := captureLogs(t, "")
	serveVia(t, http.MethodGet, "/craft/health", "")
	if buf.Len() != 0 {
		t.Fatalf("health check logged at info: %s", buf.String())
	}

	buf = captureLogs(t, "debug")
	serveVia(t, http.MethodGet, "/craft/health", "")
	if lines := logLines(t, buf); len(lines) != 1 || lines[0]["level"] != "DEBUG" {
		t.Fatalf("want one DEBUG line at debug level, got %v", lines)
	}
}

func TestAccessLog_RequestLine(t *testing.T) {
	buf := captureLogs(t, "")
	serveVia(t, http.MethodPost, "/craft/preview/mermaid/domain", `{"dsl": ""}`)

	lines := logLines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("want one line, got %v", lines)
	}
	l := lines[0]
	if l["level"] != "INFO" || l["msg"] != "request" || l["method"] != "POST" ||
		l["path"] != "/craft/preview/mermaid/domain" || l["status"] != float64(200) {
		t.Errorf("unexpected line: %v", l)
	}
	if _, ok := l["duration_ms"]; !ok {
		t.Errorf("no duration_ms: %v", l)
	}
	if l["bytes"].(float64) <= 0 {
		t.Errorf("bytes = %v, want > 0", l["bytes"])
	}
}

// A failed request is one line at warn, carrying the error message.
func TestAccessLog_ClientErrorCarriesMessage(t *testing.T) {
	buf := captureLogs(t, "")
	serveVia(t, http.MethodPost, "/craft/preview/c4", `not json`)

	lines := logLines(t, buf)
	if len(lines) != 1 {
		t.Fatalf("want one line, got %v", lines)
	}
	if l := lines[0]; l["level"] != "WARN" || l["status"] != float64(400) || l["error"] != "Invalid request format" {
		t.Errorf("unexpected line: %v", l)
	}
}

// Unmatched routes are outside the router's middleware, but still logged.
func TestAccessLog_UnmatchedRoute(t *testing.T) {
	buf := captureLogs(t, "")
	serveVia(t, http.MethodGet, "/nope", "")

	if lines := logLines(t, buf); len(lines) != 1 || lines[0]["status"] != float64(404) || lines[0]["level"] != "WARN" {
		t.Fatalf("want one WARN 404 line, got %v", lines)
	}
}

func TestNewLogger_TextByDefault(t *testing.T) {
	var buf bytes.Buffer
	newLogger(&buf, "", "bogus").Info("hello", "k", "v")
	if got := buf.String(); !strings.Contains(got, "level=INFO") || !strings.Contains(got, "k=v") {
		t.Errorf("want text output at info, got %q", got)
	}
}
