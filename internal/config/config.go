// Package config loads runtime configuration from the environment.
package config

import (
	"os"
	"strings"
	"time"
)

// Config holds all runtime settings.
type Config struct {
	Port        string
	DBPath      string
	JWTSecret   string
	JWTTTL      time.Duration
	CORSOrigins []string
}

// Load reads configuration from environment variables with sane defaults.
func Load() Config {
	return Config{
		Port:        getenv("PORT", "8080"),
		DBPath:      getenv("DB_PATH", "expense_tracker.db"),
		JWTSecret:   getenv("JWT_SECRET", "dev-insecure-secret-change-me"),
		JWTTTL:      getDuration("JWT_TTL", 24*time.Hour),
		CORSOrigins: splitCSV(getenv("CORS_ORIGINS", "*")),
	}
}

// UsesDefaultSecret reports whether the insecure default JWT secret is in use.
func (c Config) UsesDefaultSecret() bool {
	return c.JWTSecret == "dev-insecure-secret-change-me"
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
