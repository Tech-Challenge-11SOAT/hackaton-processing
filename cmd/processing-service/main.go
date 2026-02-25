package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/thiagomartins/hackaton-processing/internal/adapter/postgres"
	"github.com/thiagomartins/hackaton-processing/internal/adapter/rabbitmq"
	s3adapter "github.com/thiagomartins/hackaton-processing/internal/adapter/s3"
	"github.com/thiagomartins/hackaton-processing/internal/adapter/video"
	"github.com/thiagomartins/hackaton-processing/internal/config"
	"github.com/thiagomartins/hackaton-processing/internal/httpserver"
	"github.com/thiagomartins/hackaton-processing/internal/logging"
	"github.com/thiagomartins/hackaton-processing/internal/usecase"
	"github.com/thiagomartins/hackaton-processing/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.Log.Level)
	slog.SetDefault(logger)

	postgresPool, err := newPostgresPool(context.Background(), cfg)
	if err != nil {
		logger.Error("failed to create postgres pool", "error", err)
		os.Exit(1)
	}
	defer postgresPool.Close()

	rabbitConn, err := amqp.Dial(cfg.RabbitMQ.URL)
	if err != nil {
		logger.Error("failed to connect to rabbitmq", "error", err)
		os.Exit(1)
	}
	defer func() { _ = rabbitConn.Close() }()

	rabbitAdapter, err := rabbitmq.NewAdapter(rabbitConn, rabbitmq.Config{
		VideoProcessQueue: cfg.RabbitMQ.VideoProcessQueue,
		StatusUpdateQueue: cfg.RabbitMQ.StatusUpdateQueue,
		VideoCompletedQ:   cfg.RabbitMQ.VideoCompletedQ,
		VideoFailedQ:      cfg.RabbitMQ.VideoFailedQ,
		ConsumerTag:       cfg.RabbitMQ.ConsumerTag,
		PrefetchCount:     cfg.RabbitMQ.PrefetchCount,
	})
	if err != nil {
		logger.Error("failed to create rabbitmq adapter", "error", err)
		os.Exit(1)
	}
	defer func() { _ = rabbitAdapter.Close() }()

	s3Client, err := newS3Client(context.Background(), cfg)
	if err != nil {
		logger.Error("failed to create s3 client", "error", err)
		os.Exit(1)
	}

	inputS3Storage, err := s3adapter.NewStorage(cfg.S3.InputBucket, s3Client)
	if err != nil {
		logger.Error("failed to create input s3 storage adapter", "error", err)
		os.Exit(1)
	}

	outputS3Storage, err := s3adapter.NewStorage(cfg.S3.OutputBucket, s3Client)
	if err != nil {
		logger.Error("failed to create output s3 storage adapter", "error", err)
		os.Exit(1)
	}

	videoProcessor, err := video.NewFFmpegProcessor(cfg.Worker.FFmpegBin)
	if err != nil {
		logger.Error("failed to create ffmpeg processor", "error", err)
		os.Exit(1)
	}

	jobRepository := postgres.NewRepository(postgresPool)
	processUseCase := usecase.NewProcessVideoUseCase(
		jobRepository,
		inputS3Storage,
		outputS3Storage,
		videoProcessor,
		rabbitAdapter,
		logger,
	)
	videoWorker := worker.NewVideoProcessWorker(rabbitAdapter, processUseCase, logger)

	readinessChecker := func(ctx context.Context) error {
		if err := postgresPool.Ping(ctx); err != nil {
			return fmt.Errorf("postgres not ready: %w", err)
		}
		if rabbitConn.IsClosed() {
			return errors.New("rabbitmq not ready: connection is closed")
		}
		return nil
	}

	server := httpserver.New(cfg.HTTP, logger, readinessChecker)

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	componentErr := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Go(func() {
		if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentErr <- fmt.Errorf("http server failed: %w", err)
		}
	})

	wg.Go(func() {
		if err := videoWorker.Run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
			componentErr <- fmt.Errorf("worker failed: %w", err)
		}
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var exitErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-componentErr:
		exitErr = err
		logger.Error("component exited with error", "error", err)
	}

	runCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		if exitErr == nil {
			exitErr = err
		}
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-shutdownCtx.Done():
		logger.Warn("timeout waiting goroutines to stop", "error", shutdownCtx.Err())
	}

	logger.Info("processing service stopped gracefully")

	if exitErr != nil {
		os.Exit(1)
	}
}

func newPostgresPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.Postgres.URL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres url: %w", err)
	}

	poolConfig.MaxConns = cfg.Postgres.MaxConns
	poolConfig.MinConns = cfg.Postgres.MinConns
	poolConfig.MaxConnLifetime = cfg.Postgres.MaxConnLifetime
	poolConfig.HealthCheckPeriod = cfg.Postgres.HealthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}

func newS3Client(ctx context.Context, cfg *config.Config) (*awss3.Client, error) {
	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.S3.Region),
	}

	if cfg.S3.AccessKeyID != "" && cfg.S3.SecretAccessKey != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.S3.AccessKeyID,
				cfg.S3.SecretAccessKey,
				"",
			),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		if cfg.S3.Endpoint != "" {
			o.BaseEndpoint = &cfg.S3.Endpoint
		}
		o.UsePathStyle = cfg.S3.UsePathStyle
	}), nil
}
