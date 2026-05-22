package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	DatabaseURL           string
	HTTPAddr              string
	LockTimeout           time.Duration
	ExpirerPollInterval   time.Duration
	IdempotencyRetention  time.Duration
	ReservationTTL        time.Duration
}

// MustLoad reads configuration from the environment and applies sane defaults.
// It panics on a missing required value to fail loudly at startup —
// consistency-over-availability extends to configuration.
func MustLoad() *Config {
	cfg := &Config{
		DatabaseURL:          getenv("DATABASE_URL", "postgres://flashsale:flashsale@localhost:5432/flashsale?sslmode=disable"),
		HTTPAddr:             getenv("HTTP_ADDR", ":8080"),
		LockTimeout:          getenvDuration("LOCK_TIMEOUT", 2*time.Second),
		ExpirerPollInterval:  getenvDuration("EXPIRER_POLL_INTERVAL", 500*time.Millisecond),
		IdempotencyRetention: getenvDuration("IDEMPOTENCY_RETENTION", 24*time.Hour),
		ReservationTTL:       getenvDuration("RESERVATION_TTL", 60*time.Second),
	}
	if cfg.DatabaseURL == "" {
		panic("config: DATABASE_URL is required")
	}
	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		panic(fmt.Sprintf("config: %s is not a valid duration: %v", key, err))
	}
	return d
}
