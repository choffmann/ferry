package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestNewLoggerWritesJSONLines(t *testing.T) {
	var buf bytes.Buffer
	NewLogger(&buf).Info("hallo", "request_id", "abc123")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, buf.String())
	}
	if line["msg"] != "hallo" {
		t.Errorf("msg = %v, want hallo", line["msg"])
	}
	if line["request_id"] != "abc123" {
		t.Errorf("request_id = %v, want abc123", line["request_id"])
	}
	if line["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", line["level"])
	}
}

func TestLoggerRoundTripsThroughTheContext(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	ctx := WithLogger(context.Background(), l)
	if got := LoggerFrom(ctx); got != l {
		t.Error("LoggerFrom returned a different logger")
	}
}

func TestLoggerFromWithoutOneNeverReturnsNil(t *testing.T) {
	if LoggerFrom(context.Background()) == nil {
		t.Error("LoggerFrom on a bare context returned nil")
	}
}
