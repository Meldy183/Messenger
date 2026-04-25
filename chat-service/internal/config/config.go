package config

import (
	"errors"
	"os"
)

type Config struct {
	Env       string
	Port      string
	DBDSN     string
	JWTSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:       getEnv("ENV", "development"),
		Port:      getEnv("PORT", "8081"),
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
