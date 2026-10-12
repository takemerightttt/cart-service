package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	LogLevel        slog.Level
	DBMaxConns      int32
	ShutdownTimeout time.Duration
	IdempotencyTTL  time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		ShutdownTimeout: 10 * time.Second,
		IdempotencyTTL:  24 * time.Hour,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: DATABASE_URL is required")
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(getenv("LOG_LEVEL", "info"))); err != nil {
		return Config{}, fmt.Errorf("config: LOG_LEVEL: %w", err)
	}
	maxConns, err := strconv.ParseInt(getenv("DB_MAX_CONNS", "10"), 10, 32)
	if err != nil || maxConns < 1 {
		return Config{}, fmt.Errorf("config: DB_MAX_CONNS must be a positive integer")
	}
	cfg.DBMaxConns = int32(maxConns)
	return cfg, nil
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
