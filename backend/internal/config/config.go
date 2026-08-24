// Package config loads application configuration from environment
// variables, optionally seeded from a .env file in the working directory.
// All secrets (DB credentials, future JWT keys) live in the environment,
// never in code.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	App    App
	HTTP   HTTP
	DB     DB
	Auth   Auth
	Metals Metals
}

type Metals struct {
	// APIKey enables the live price refresher; empty keeps seed/manual
	// prices with staleness visible to clients.
	APIKey          string        `env:"METALS_API_KEY"`
	APIURL          string        `env:"METALS_API_URL" env-default:"https://api.metals.dev/v1/latest"`
	Currency        string        `env:"METALS_CURRENCY" env-default:"UZS"`
	RefreshInterval time.Duration `env:"METALS_REFRESH_INTERVAL" env-default:"6h"`
	StaleAfter      time.Duration `env:"METALS_STALE_AFTER" env-default:"48h"`
}

type Auth struct {
	// JWTSecret signs access tokens. Required; no default on purpose —
	// a secret must come from the environment, never from code.
	JWTSecret  string        `env:"JWT_SECRET"`
	AccessTTL  time.Duration `env:"AUTH_ACCESS_TTL" env-default:"15m"`
	RefreshTTL time.Duration `env:"AUTH_REFRESH_TTL" env-default:"720h"`
}

type App struct {
	Env      string `env:"APP_ENV" env-default:"dev"`
	LogLevel string `env:"LOG_LEVEL" env-default:"info"`
}

type HTTP struct {
	Port            int           `env:"HTTP_PORT" env-default:"8080"`
	ReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT" env-default:"10s"`
	WriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT" env-default:"15s"`
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" env-default:"10s"`
	MaxBodyBytes    int64         `env:"HTTP_MAX_BODY_BYTES" env-default:"65536"`
	RateLimitPerMin int           `env:"RATE_LIMIT_PER_MIN" env-default:"120"`
}

type DB struct {
	Host     string `env:"DB_HOST" env-default:"localhost"`
	Port     int    `env:"DB_PORT" env-default:"5432"`
	User     string `env:"DB_USER" env-default:"islamic"`
	Password string `env:"DB_PASSWORD"`
	Name     string `env:"DB_NAME" env-default:"islamiccalculator"`
	SSLMode  string `env:"DB_SSLMODE" env-default:"disable"`
}

// DSN returns the PostgreSQL connection string.
func (d DB) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode)
}

// Load reads configuration from the process environment, seeded from
// envFile when it exists. Real environment variables always win over
// file values (godotenv.Load never overrides existing vars).
func Load(envFile string) (*Config, error) {
	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Load(envFile); err != nil {
			return nil, fmt.Errorf("read %s: %w", envFile, err)
		}
	}
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return nil, fmt.Errorf("read env: %w", err)
	}
	return &cfg, nil
}
