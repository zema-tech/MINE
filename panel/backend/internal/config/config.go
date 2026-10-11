package config

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"strconv"
)

// Config holds all panel runtime configuration from env.
type Config struct {
	Port          string
	AppURL        string
	DatabaseURL   string
	RedisAddr     string
	RedisPassword string
	JWTSecret     string
	// EncryptionKey is 32 bytes for AES-256-GCM (secrets at rest).
	EncryptionKey string
	AdminEmail    string
	AdminPassword string
}

// Load reads configuration from environment with sane dev defaults.
func Load() Config {
	return Config{
		Port:          envOr("MINE_PORT", "8080"),
		AppURL:        envOr("MINE_APP_URL", "http://localhost:8080"),
		DatabaseURL:   envOr("MINE_DATABASE_URL", "postgres://mine:mine_dev_secret@localhost:5432/mine?sslmode=disable"),
		RedisAddr:     envOr("MINE_REDIS_ADDR", "localhost:6379"),
		RedisPassword: os.Getenv("MINE_REDIS_PASSWORD"),
		JWTSecret:     envOr("MINE_JWT_SECRET", "dev_jwt_secret_change_me_32chars_min"),
		EncryptionKey: envOr("MINE_ENCRYPTION_KEY", "dev_encryption_key_32bytes_xxxx"),
		AdminEmail:    envOr("MINE_ADMIN_EMAIL", "admin@example.com"),
		AdminPassword: envOr("MINE_ADMIN_PASSWORD", "ChangeMe123!"),
	}
}

// RateLimit holds per-route limits (requests per minute).
type RateLimit struct {
	Auth  int
	API   int
	Power int
}

func DefaultRateLimit() RateLimit { return RateLimit{Auth: 20, API: 300, Power: 30} }

func HTTPPort(c Config) int {
	p, err := strconv.Atoi(c.Port)
	if err != nil {
		return 8080
	}
	return p
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// RandomToken generates a URL-safe random token of n bytes.
func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
