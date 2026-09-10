package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests run in cmd/ferry, so ASSETS_DIR points at the directory in the
// repository root unless the case sets it itself.
func env(vars map[string]string) func(string) string {
	return func(k string) string {
		if v, ok := vars[k]; ok {
			return v
		}
		if k == "ASSETS_DIR" {
			return filepath.Join("..", "..", "assets")
		}
		return ""
	}
}

func TestRunServeAnswersAndShutsDown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var logs bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- runServe(ctx, []string{"-addr", "127.0.0.1:18081"}, env(nil), &logs)
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
		done <- runServe(ctx, []string{"-addr", "127.0.0.1:18082"}, env(nil), &logs)
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
		done <- runServe(ctx, []string{"-addr", "127.0.0.1:18083"}, env(nil), &logs)
	}()
	waitForHealth(t, "http://127.0.0.1:18083/healthz")
	cancel()
	<-done

	if !warnedAbout(logs.String(), "-ldflags") {
		t.Errorf("no warning about the missing build stamp:\n%s", logs.String())
	}
}

func TestRunServeRefusesToStartWithoutTheAssets(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "assets")
	err := runServe(context.Background(), nil,
		env(map[string]string{"ASSETS_DIR": missing}), io.Discard)
	if err == nil {
		t.Fatal("runServe with a missing asset directory returned no error")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q does not name the asset directory it looked in", err)
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
		env(map[string]string{"PORT": "achttausend"}), io.Discard)
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
