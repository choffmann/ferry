package config

import "testing"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadUsesDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != DefaultPort {
		t.Errorf("Port = %d, want %d", c.Port, DefaultPort)
	}
	if c.AdminToken != DefaultAdminToken {
		t.Errorf("AdminToken = %q, want %q", c.AdminToken, DefaultAdminToken)
	}
	if !c.AdminTokenIsDefault {
		t.Error("AdminTokenIsDefault = false, want true")
	}
}

func TestLoadReadsTheEnvironment(t *testing.T) {
	c, err := Load(env(map[string]string{"PORT": "3000", "ADMIN_TOKEN": "s3cret"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != 3000 {
		t.Errorf("Port = %d, want 3000", c.Port)
	}
	if c.AdminToken != "s3cret" {
		t.Errorf("AdminToken = %q, want s3cret", c.AdminToken)
	}
	if c.AdminTokenIsDefault {
		t.Error("AdminTokenIsDefault = true although the token was set")
	}
}

func TestLoadRejectsAnUnusablePort(t *testing.T) {
	for _, v := range []string{"achttausend", "0", "-1", "70000"} {
		if _, err := Load(env(map[string]string{"PORT": v})); err == nil {
			t.Errorf("Load with PORT=%q returned no error", v)
		}
	}
}

func TestLoadTreatsAnEmptyValueAsUnset(t *testing.T) {
	c, err := Load(env(map[string]string{"PORT": "", "ADMIN_TOKEN": ""}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Port != DefaultPort || c.AdminToken != DefaultAdminToken {
		t.Errorf("empty values did not fall back to the defaults: %+v", c)
	}
}
