package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func required(extra map[string]string) map[string]string {
	m := map[string]string{
		"DATABASE_URL": "postgres://ferry@localhost/ferry",
		"ADMIN_TOKEN":  "s3cret",
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestLoadFallsBackForPortAndAssets(t *testing.T) {
	c, err := Load(env(required(nil)))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", c.Port, DefaultPort)
	}
	if c.AssetsDir != DefaultAssetsDir {
		t.Errorf("AssetsDir = %q, want %q", c.AssetsDir, DefaultAssetsDir)
	}
}

func TestLoadReadsTheEnvironment(t *testing.T) {
	c, err := Load(env(required(map[string]string{
		"PORT":       "3000",
		"ASSETS_DIR": "/srv/ferry/assets",
	})))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != 3000 {
		t.Errorf("Port = %d, want 3000", c.Port)
	}
	if c.AdminToken != "s3cret" {
		t.Errorf("AdminToken = %q, want s3cret", c.AdminToken)
	}
	if c.DatabaseURL != "postgres://ferry@localhost/ferry" {
		t.Errorf("DatabaseURL = %q", c.DatabaseURL)
	}
	if c.AssetsDir != "/srv/ferry/assets" {
		t.Errorf("AssetsDir = %q, want /srv/ferry/assets", c.AssetsDir)
	}
}

func TestLoadNamesTheMissingVariable(t *testing.T) {
	for _, missing := range []string{"DATABASE_URL", "ADMIN_TOKEN"} {
		m := required(nil)
		delete(m, missing)

		_, err := Load(env(m))
		if err == nil {
			t.Errorf("Load without %s returned no error", missing)
			continue
		}
		if !strings.Contains(err.Error(), missing) {
			t.Errorf("error %q does not name %s", err, missing)
		}
	}
}

func TestLoadRejectsAnUnusablePort(t *testing.T) {
	for _, v := range []string{"achttausend", "0", "-1", "70000"} {
		if _, err := Load(env(required(map[string]string{"PORT": v}))); err == nil {
			t.Errorf("Load with PORT=%q returned no error", v)
		}
	}
}

func TestLoadTreatsAnEmptyValueAsUnset(t *testing.T) {
	c, err := Load(env(required(map[string]string{"PORT": "", "ASSETS_DIR": ""})))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != DefaultPort || c.AssetsDir != DefaultAssetsDir {
		t.Errorf("empty values did not fall back to the defaults: %+v", c)
	}
}

// migrate and seed get by with the database alone, and the migration service
// should not be handed a token it has no use for.
func TestDatabaseURLDoesNotNeedTheAdminToken(t *testing.T) {
	url, err := DatabaseURL(env(map[string]string{"DATABASE_URL": "postgres://ferry@localhost/ferry"}))
	if err != nil {
		t.Fatalf("DatabaseURL: %v", err)
	}
	if url != "postgres://ferry@localhost/ferry" {
		t.Errorf("DatabaseURL = %q", url)
	}

	if _, err := DatabaseURL(env(nil)); err == nil {
		t.Error("DatabaseURL without DATABASE_URL returned no error")
	}
}

func TestPaymentsDefaultsToPortEightyEighty(t *testing.T) {
	cfg, err := Payments(func(string) string { return "" })
	if err != nil {
		t.Fatalf("Payments: %v", err)
	}
	if cfg.Port != DefaultPort {
		t.Errorf("port = %d, want %d", cfg.Port, DefaultPort)
	}
}

func TestPaymentsRejectsAPortThatIsNotANumber(t *testing.T) {
	_, err := Payments(func(k string) string {
		if k == "PORT" {
			return "eighty"
		}
		return ""
	})
	if err == nil {
		t.Error("a PORT of \"eighty\" was accepted")
	}
}

func TestWorkerNamesTheVariableItIsMissing(t *testing.T) {
	env := map[string]string{"DATABASE_URL": "postgres://localhost/ferry"}
	_, err := Worker(func(k string) string { return env[k] })
	if err == nil {
		t.Fatal("a worker without PAYMENTS_URL was accepted")
	}
	if !strings.Contains(err.Error(), "PAYMENTS_URL") {
		t.Errorf("the error does not name PAYMENTS_URL: %v", err)
	}
}

func TestWorkerReadsBothAddresses(t *testing.T) {
	env := map[string]string{
		"DATABASE_URL": "postgres://localhost/ferry",
		"PAYMENTS_URL": "http://payments:8080",
	}
	cfg, err := Worker(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("Worker: %v", err)
	}
	if cfg.DatabaseURL != env["DATABASE_URL"] || cfg.PaymentsURL != env["PAYMENTS_URL"] {
		t.Errorf("config = %+v, want both addresses", cfg)
	}
}
