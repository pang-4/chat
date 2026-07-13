package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Server     Server
	Database   Database
	Migrations Migrations
}

type Server struct {
	Address        string
	AllowedOrigins []string
}

type Database struct {
	Host               string
	Port               string
	User               string
	Password           string
	Name               string
	SSLMode            string
	MaxOpenConnections int
	MaxIdleConnections int
}

type Migrations struct {
	SourceURL string
}

func Load() (Config, error) {
	// A .env file is convenient locally, while containers inject real environment variables.
	_ = godotenv.Load()

	maxOpen, err := intFromEnv("DB_MAX_OPEN_CONNS", 25)
	if err != nil {
		return Config{}, err
	}
	maxIdle, err := intFromEnv("DB_MAX_IDLE_CONNS", 10)
	if err != nil {
		return Config{}, err
	}
	if maxOpen < 1 || maxIdle < 0 || maxIdle > maxOpen {
		return Config{}, fmt.Errorf("invalid database pool settings: max_idle must be between 0 and max_open")
	}

	cfg := Config{
		Server: Server{
			Address:        valueOrDefault("SERVER_ADDR", ":8080"),
			AllowedOrigins: splitCSV(valueOrDefault("WS_ALLOWED_ORIGINS", "*")),
		},
		Database: Database{
			Host:               valueOrDefault("DB_HOST", "localhost"),
			Port:               valueOrDefault("DB_PORT", "5432"),
			User:               valueOrDefault("DB_USER", "postgres"),
			Password:           os.Getenv("DB_PASSWORD"),
			Name:               valueOrDefault("DB_NAME", "chat"),
			SSLMode:            valueOrDefault("DB_SSLMODE", "disable"),
			MaxOpenConnections: maxOpen,
			MaxIdleConnections: maxIdle,
		},
		Migrations: Migrations{
			SourceURL: valueOrDefault("MIGRATIONS_SOURCE_URL", "file://migrations"),
		},
	}

	if cfg.Database.Password == "" {
		return Config{}, fmt.Errorf("DB_PASSWORD is required")
	}

	return cfg, nil
}

func valueOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func intFromEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
