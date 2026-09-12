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

type PaymentsConfig struct {
	Port int
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

	p, err := port(getenv)
	if err != nil {
		return Config{}, err
	}

	c := Config{
		Port:        p,
		AdminToken:  token,
		AssetsDir:   DefaultAssetsDir,
		DatabaseURL: url,
	}

	if raw := getenv("ASSETS_DIR"); raw != "" {
		c.AssetsDir = raw
	}

	return c, nil
}

// Payments is the whole configuration of the second service. It needs neither
// a database nor a token.
func Payments(getenv func(string) string) (PaymentsConfig, error) {
	p, err := port(getenv)
	if err != nil {
		return PaymentsConfig{}, err
	}
	return PaymentsConfig{Port: p}, nil
}

func port(getenv func(string) string) (int, error) {
	raw := getenv("PORT")
	if raw == "" {
		return DefaultPort, nil
	}
	p, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("PORT is not a number: %q", raw)
	}
	if p < 1 || p > 65535 {
		return 0, fmt.Errorf("PORT is outside 1..65535: %d", p)
	}
	return p, nil
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
