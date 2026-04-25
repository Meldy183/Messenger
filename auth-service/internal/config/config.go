package config

import (
	"errors"
	"os"
)

type Config struct {
	Env       string // "development" | "production"
	Port      string // HTTP listen port, e.g. "8080"
	DBDSN     string // Postgres connection string
	JWTSecret string // HS256 signing key, shared with other services via K8s Secret
}

// Load reads configuration from environment variables.
// Required vars missing → error so main() can exit with a clear message.
func Load() (*Config, error) {
	cfg := &Config{
		Env:       getEnv("ENV", "development"),
		Port:      getEnv("PORT", "8080"),
		DBDSN:     os.Getenv("DATABASE_URL"),
		JWTSecret: os.Getenv("JWT_SECRET"),
	}

	var errs []error
	if cfg.DBDSN == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if cfg.JWTSecret == "" {
		errs = append(errs, errors.New("JWT_SECRET is required"))
	}

	return cfg, errors.Join(errs...)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
