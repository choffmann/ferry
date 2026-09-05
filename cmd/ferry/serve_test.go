package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRunServeAnswersAndShutsDown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var logs bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- runServe(ctx, []string{"-addr", "127.0.0.1:18081"}, func(string) string { return "" }, &logs)
	}()

	waitForHealth(t, "http://127.0.0.1:18081/healthz")

	resp, err := http.Get("http://127.0.0.1:18081/connections")
	if err != nil {
		t.Fatalf("GET /connections: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body %s", resp.StatusCode, body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runServe: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runServe did not return after the context was cancelled")
	}
}

func TestRunServeWarnsAboutTheDefaultToken(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var logs bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- runServe(ctx, []string{"-addr", "127.0.0.1:18082"}, func(string) string { return "" }, &logs)
	}()
	waitForHealth(t, "http://127.0.0.1:18082/healthz")
	cancel()
	<-done

	var warned bool
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		msg, _ := entry["msg"].(string)
		if entry["level"] == "WARN" && strings.Contains(msg, "ADMIN_TOKEN") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("no warning about the default admin token:\n%s", logs.String())
	}
}

func TestRunServeRejectsABadPort(t *testing.T) {
	err := runServe(context.Background(), nil,
		func(k string) string {
			if k == "PORT" {
				return "achttausend"
			}
			return ""
		}, io.Discard)
	if err == nil {
		t.Error("runServe with PORT=achttausend returned no error")
	}
}

func waitForHealth(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s never became healthy", url)
}
