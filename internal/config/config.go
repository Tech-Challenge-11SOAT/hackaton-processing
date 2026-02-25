package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config groups all service configurations.
type Config struct {
	HTTP     HTTPConfig
	Log      LogConfig
	Postgres PostgresConfig
	RabbitMQ RabbitMQConfig
	S3       S3Config
	Worker   WorkerConfig
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

// PostgresConfig contains PostgreSQL connectivity settings.
type PostgresConfig struct {
	URL               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	HealthCheckPeriod time.Duration
}

// RabbitMQConfig contains broker connectivity and queue settings.
type RabbitMQConfig struct {
	URL               string
	VideoProcessQueue string
	StatusUpdateQueue string
	VideoCompletedQ   string
	VideoFailedQ      string
	ConsumerTag       string
	PrefetchCount     int
}

// S3Config contains object storage settings.
type S3Config struct {
	Region          string
	InputBucket     string
	OutputBucket    string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

// WorkerConfig contains worker runtime settings.
type WorkerConfig struct {
	Concurrency int
	FFmpegBin   string
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

	postgresMaxConns, err := getIntEnv("POSTGRES_MAX_CONNS", 10)
	if err != nil {
		return nil, err
	}
	postgresMinConns, err := getIntEnv("POSTGRES_MIN_CONNS", 2)
	if err != nil {
		return nil, err
	}
	postgresMaxConnLifetime, err := getDurationEnv("POSTGRES_MAX_CONN_LIFETIME", "30m")
	if err != nil {
		return nil, err
	}
	postgresHealthCheckPeriod, err := getDurationEnv("POSTGRES_HEALTHCHECK_PERIOD", "30s")
	if err != nil {
		return nil, err
	}
	prefetchCount, err := getIntEnv("RABBITMQ_PREFETCH_COUNT", 1)
	if err != nil {
		return nil, err
	}
	workerConcurrency, err := getIntEnv("WORKER_CONCURRENCY", 1)
	if err != nil {
		return nil, err
	}
	s3UsePathStyle, err := getBoolEnv("S3_USE_PATH_STYLE", false)
	if err != nil {
		return nil, err
	}

	postgresURL, err := getRequiredEnv("POSTGRES_URL")
	if err != nil {
		return nil, err
	}
	rabbitMQURL, err := getRequiredEnv("RABBITMQ_URL")
	if err != nil {
		return nil, err
	}
	s3Region, err := getRequiredEnv("S3_REGION")
	if err != nil {
		return nil, err
	}
	s3InputBucket, err := getRequiredEnv("S3_INPUT_BUCKET")
	if err != nil {
		return nil, err
	}
	s3OutputBucket, err := getRequiredEnv("S3_OUTPUT_BUCKET")
	if err != nil {
		return nil, err
	}

	if postgresMaxConns < 1 {
		return nil, fmt.Errorf("POSTGRES_MAX_CONNS must be >= 1")
	}
	if postgresMinConns < 0 {
		return nil, fmt.Errorf("POSTGRES_MIN_CONNS must be >= 0")
	}
	if postgresMinConns > postgresMaxConns {
		return nil, fmt.Errorf("POSTGRES_MIN_CONNS cannot be greater than POSTGRES_MAX_CONNS")
	}
	if prefetchCount < 1 {
		return nil, fmt.Errorf("RABBITMQ_PREFETCH_COUNT must be >= 1")
	}
	if workerConcurrency < 1 {
		return nil, fmt.Errorf("WORKER_CONCURRENCY must be >= 1")
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
		Postgres: PostgresConfig{
			URL:               postgresURL,
			MaxConns:          int32(postgresMaxConns),
			MinConns:          int32(postgresMinConns),
			MaxConnLifetime:   postgresMaxConnLifetime,
			HealthCheckPeriod: postgresHealthCheckPeriod,
		},
		RabbitMQ: RabbitMQConfig{
			URL:               rabbitMQURL,
			VideoProcessQueue: getEnv("RABBITMQ_QUEUE_VIDEO_PROCESS", "video.process"),
			StatusUpdateQueue: getEnv("RABBITMQ_QUEUE_STATUS_UPDATE", "status.update"),
			VideoCompletedQ:   getEnv("RABBITMQ_QUEUE_VIDEO_COMPLETED", "video.completed"),
			VideoFailedQ:      getEnv("RABBITMQ_QUEUE_VIDEO_FAILED", "video.failed"),
			ConsumerTag:       getEnv("RABBITMQ_CONSUMER_TAG", "processing-service-consumer"),
			PrefetchCount:     prefetchCount,
		},
		S3: S3Config{
			Region:          s3Region,
			InputBucket:     s3InputBucket,
			OutputBucket:    s3OutputBucket,
			Endpoint:        getEnv("S3_ENDPOINT", ""),
			AccessKeyID:     getEnv("S3_ACCESS_KEY_ID", ""),
			SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY", ""),
			UsePathStyle:    s3UsePathStyle,
		},
		Worker: WorkerConfig{
			Concurrency: workerConcurrency,
			FFmpegBin:   getEnv("WORKER_FFMPEG_BIN", "ffmpeg"),
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

func getRequiredEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func getDurationEnv(key, fallback string) (time.Duration, error) {
	raw := getEnv(key, fallback)
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid duration in %s: %w", key, err)
	}

	return value, nil
}

func getIntEnv(key string, fallback int) (int, error) {
	raw := getEnv(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid integer in %s: %w", key, err)
	}
	return value, nil
}

func getBoolEnv(key string, fallback bool) (bool, error) {
	raw := strings.ToLower(strings.TrimSpace(getEnv(key, strconv.FormatBool(fallback))))
	switch raw {
	case "1", "true", "t", "yes", "y", "on":
		return true, nil
	case "0", "false", "f", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean in %s: %q", key, raw)
	}
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
