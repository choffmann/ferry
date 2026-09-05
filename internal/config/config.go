package config

import (
	"fmt"
	"strconv"
)

const (
	DefaultPort       = 8080
	DefaultAdminToken = "local-dev"
)

type Config struct {
	Port       int
	AdminToken string
	// AdminTokenIsDefault drives the warning on startup. v0.3.0 removes the
	// default, and by then the warning has been in the logs for weeks.
	AdminTokenIsDefault bool
}

// Load takes the lookup function instead of calling os.Getenv, so every case is
// testable without touching the process environment.
func Load(getenv func(string) string) (Config, error) {
	c := Config{Port: DefaultPort, AdminToken: DefaultAdminToken, AdminTokenIsDefault: true}

	if raw := getenv("PORT"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("PORT ist keine Zahl: %q", raw)
		}
		if port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("PORT liegt außerhalb 1..65535: %d", port)
		}
		c.Port = port
	}

	if raw := getenv("ADMIN_TOKEN"); raw != "" {
		c.AdminToken = raw
		c.AdminTokenIsDefault = false
	}

	return c, nil
}
