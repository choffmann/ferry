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

	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	resp, err := client.Get("http://127.0.0.1:18081/connections")
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
	case <-time.After(15 * time.Second):
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

	if !warnedAbout(logs.String(), "ADMIN_TOKEN") {
		t.Errorf("no warning about the default admin token:\n%s", logs.String())
	}
}

func TestRunServeWarnsAboutAnUnstampedBinary(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var logs bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- runServe(ctx, []string{"-addr", "127.0.0.1:18083"}, func(string) string { return "" }, &logs)
	}()
	waitForHealth(t, "http://127.0.0.1:18083/healthz")
	cancel()
	<-done

	if !warnedAbout(logs.String(), "-ldflags") {
		t.Errorf("no warning about the missing build stamp:\n%s", logs.String())
	}
}

func warnedAbout(logs, substring string) bool {
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		msg, _ := entry["msg"].(string)
		if entry["level"] == "WARN" && strings.Contains(msg, substring) {
			return true
		}
	}
	return false
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
