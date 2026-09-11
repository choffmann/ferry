package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/choffmann/ferry/internal/pgtest"
)

// These tests run in cmd/ferry, so ASSETS_DIR points at the directory in the
// repository root unless the case sets it itself. A case that wants a variable
// gone overrides it with the empty string.
func env(vars map[string]string) func(string) string {
	base := map[string]string{
		"ADMIN_TOKEN":  "test-token",
		"DATABASE_URL": databaseURL(),
		"ASSETS_DIR":   filepath.Join("..", "..", "assets"),
	}
	for k, v := range vars {
		base[k] = v
	}
	return func(k string) string { return base[k] }
}

const unreachableDatabase = "postgres://ferry:ferry@127.0.0.1:1/ferry?sslmode=disable&connect_timeout=2"

// Cases that only check the configuration need a well-formed address, not a
// database behind it.
func databaseURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return unreachableDatabase
}

// readyDatabase is what the compose file of a team has to arrange too: schema
// first, then data, then the server.
func readyDatabase(t *testing.T) string {
	t.Helper()

	dsn := pgtest.DSN(t)
	e := env(map[string]string{"DATABASE_URL": dsn})
	if err := runMigrate(context.Background(), nil, e, io.Discard); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := runSeed(context.Background(), nil, e, io.Discard); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return dsn
}

func TestRunServeAnswersAndShutsDown(t *testing.T) {
	e := env(map[string]string{"DATABASE_URL": readyDatabase(t)})
	ctx, cancel := context.WithCancel(context.Background())
	var logs bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- runServe(ctx, []string{"-addr", "127.0.0.1:18081"}, e, &logs)
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
	if !strings.Contains(string(body), "FL-SO") {
		t.Errorf("the seeded connections are missing from %s", body)
	}

	ready, err := client.Get("http://127.0.0.1:18081/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	ready.Body.Close()
	if ready.StatusCode != http.StatusOK {
		t.Errorf("readyz = %d with a database behind it, want 200", ready.StatusCode)
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

func TestRunServeStopsWhenARequiredVariableIsMissing(t *testing.T) {
	for _, name := range []string{"ADMIN_TOKEN", "DATABASE_URL"} {
		err := runServe(context.Background(), nil,
			env(map[string]string{name: ""}), io.Discard)
		if err == nil {
			t.Errorf("runServe without %s returned no error", name)
			continue
		}
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

func TestRunServeWarnsAboutAnUnstampedBinary(t *testing.T) {
	e := env(map[string]string{"DATABASE_URL": readyDatabase(t)})
	ctx, cancel := context.WithCancel(context.Background())
	var logs bytes.Buffer

	done := make(chan error, 1)
	go func() {
		done <- runServe(ctx, []string{"-addr", "127.0.0.1:18083"}, e, &logs)
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

// Waiting for the database is the job of the topology, so an unreachable one is
// a startup error. The timeout catches the opposite outcome, a server that came
// up regardless and would otherwise run until the test binary is killed.
func TestRunServeStopsWhenTheDatabaseIsUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := runServe(ctx, []string{"-addr", "127.0.0.1:18084"},
		env(map[string]string{"DATABASE_URL": unreachableDatabase}), io.Discard)
	if err == nil {
		t.Fatal("runServe without a reachable database returned no error")
	}
}
