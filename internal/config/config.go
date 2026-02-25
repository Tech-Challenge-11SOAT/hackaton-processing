package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config groups all service configurations.
type Config struct {
	HTTP HTTPConfig
	Log  LogConfig
}

// HTTPConfig contains HTTP server settings.
type HTTPConfig struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// LogConfig contains logger settings.
type LogConfig struct {
	Level slog.Level
}

// Load reads and validates configuration from environment variables.
func Load() (*Config, error) {
	if err := loadDotEnv(); err != nil {
		return nil, err
	}

	readTimeout, err := getDurationEnv("HTTP_READ_TIMEOUT", "10s")
	if err != nil {
		return nil, err
	}

	writeTimeout, err := getDurationEnv("HTTP_WRITE_TIMEOUT", "10s")
	if err != nil {
		return nil, err
	}

	idleTimeout, err := getDurationEnv("HTTP_IDLE_TIMEOUT", "30s")
	if err != nil {
		return nil, err
	}

	shutdownTimeout, err := getDurationEnv("HTTP_SHUTDOWN_TIMEOUT", "15s")
	if err != nil {
		return nil, err
	}

	level, err := parseLogLevel(getEnv("LOG_LEVEL", "info"))
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTP: HTTPConfig{
			Port:            getEnv("HTTP_PORT", "8080"),
			ReadTimeout:     readTimeout,
			WriteTimeout:    writeTimeout,
			IdleTimeout:     idleTimeout,
			ShutdownTimeout: shutdownTimeout,
		},
		Log: LogConfig{
			Level: level,
		},
	}, nil
}

func loadDotEnv() error {
	err := godotenv.Load(".env")
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return fmt.Errorf("failed to load .env file: %w", err)
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func getDurationEnv(key, fallback string) (time.Duration, error) {
	raw := getEnv(key, fallback)
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid duration in %s: %w", key, err)
	}

	return value, nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid LOG_LEVEL value: %q", raw)
	}
}
