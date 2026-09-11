package config

import (
	"fmt"
	"strconv"
)

const (
	DefaultPort      = 8080
	DefaultAssetsDir = "./assets"
)

type Config struct {
	Port int
	// AdminToken has no default any more. A process that comes up with a known
	// password is worse than one that refuses to come up at all.
	AdminToken string
	// AssetsDir holds the templates the application reads at startup. Relative
	// to the working directory, so a container needs both the binary and the
	// directory.
	AssetsDir   string
	DatabaseURL string
}

// Load takes the lookup function instead of calling os.Getenv, so every case is
// testable without touching the process environment.
func Load(getenv func(string) string) (Config, error) {
	url, err := DatabaseURL(getenv)
	if err != nil {
		return Config{}, err
	}

	token := getenv("ADMIN_TOKEN")
	if token == "" {
		return Config{}, missing("ADMIN_TOKEN", "the bearer token of the chaos interface")
	}

	c := Config{
		Port:        DefaultPort,
		AdminToken:  token,
		AssetsDir:   DefaultAssetsDir,
		DatabaseURL: url,
	}

	if raw := getenv("PORT"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("PORT is not a number: %q", raw)
		}
		if port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("PORT is outside 1..65535: %d", port)
		}
		c.Port = port
	}

	if raw := getenv("ASSETS_DIR"); raw != "" {
		c.AssetsDir = raw
	}

	return c, nil
}

// DatabaseURL stands on its own because migrate and seed need the database and
// nothing else. The migration service is not handed a token it has no use for.
func DatabaseURL(getenv func(string) string) (string, error) {
	url := getenv("DATABASE_URL")
	if url == "" {
		return "", missing("DATABASE_URL", "the connection string of the database")
	}
	return url, nil
}

func missing(name, what string) error {
	return fmt.Errorf("%s is not set, it carries %s", name, what)
}
