package config

import (
	"errors"
	"os"
)

type Config struct {
	Env            string
	DBDSN          string
	KafkaBrokers   string
	KafkaGroupID   string
	ChatServiceURL string // base URL of chat-service, e.g. http://chat-service:8081
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:            getEnv("ENV", "development"),
		DBDSN:          os.Getenv("DATABASE_URL"),
		KafkaBrokers:   getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaGroupID:   getEnv("KAFKA_GROUP_ID", "message-worker"),
		ChatServiceURL: getEnv("CHAT_SERVICE_URL", "http://localhost:8081"),
	}

	var errs []error
	if cfg.DBDSN == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	return cfg, errors.Join(errs...)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
