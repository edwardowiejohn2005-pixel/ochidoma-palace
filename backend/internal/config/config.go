// Package config loads runtime configuration from environment variables.
// No secrets are ever hardcoded here — see .env.example for the full list.
package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Env                string // "development" | "production"
	Port               string
	DatabaseURL        string
	JWTAccessSecret    string
	JWTRefreshSecret   string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	MediaStoragePath   string
	AllowedOrigins     []string
	MaxUploadSizeBytes int64
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:                getEnv("APP_ENV", "development"),
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTAccessSecret:    os.Getenv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret:   os.Getenv("JWT_REFRESH_SECRET"),
		AccessTokenTTL:     15 * time.Minute,
		RefreshTokenTTL:    7 * 24 * time.Hour,
		MediaStoragePath:   getEnv("MEDIA_STORAGE_PATH", "./media"),
		MaxUploadSizeBytes: 25 * 1024 * 1024, // 25MB default
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTAccessSecret == "" || cfg.JWTRefreshSecret == "" {
		return nil, fmt.Errorf("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET are required")
	}
	if len(cfg.JWTAccessSecret) < 32 || len(cfg.JWTRefreshSecret) < 32 {
		return nil, fmt.Errorf("JWT secrets must be at least 32 characters — generate with `openssl rand -base64 48`")
	}

	origins := getEnv("ALLOWED_ORIGINS", "http://localhost:3000")
	cfg.AllowedOrigins = splitCSV(origins)

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}
