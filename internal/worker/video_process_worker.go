package worker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/thiagomartins/hackaton-processing/internal/port"
)

// VideoProcessExecutor abstracts the processing use case execution.
type VideoProcessExecutor interface {
	Execute(ctx context.Context, message port.VideoProcessMessage) error
}

// VideoProcessWorker binds queue consumption to the processing use case.
type VideoProcessWorker struct {
	consumer port.VideoProcessConsumer
	useCase  VideoProcessExecutor
	logger   *slog.Logger
}

// NewVideoProcessWorker creates a new worker instance.
func NewVideoProcessWorker(
	consumer port.VideoProcessConsumer,
	useCase VideoProcessExecutor,
	logger *slog.Logger,
) *VideoProcessWorker {
	if logger == nil {
		logger = slog.Default()
	}
	return &VideoProcessWorker{
		consumer: consumer,
		useCase:  useCase,
		logger:   logger,
	}
}

// Run starts consuming video.process messages until context cancellation.
func (w *VideoProcessWorker) Run(ctx context.Context) error {
	if w.consumer == nil {
		return fmt.Errorf("consumer is required")
	}
	if w.useCase == nil {
		return fmt.Errorf("use case is required")
	}

	w.logger.Info("starting video.process worker")

	return w.consumer.ConsumeVideoProcess(ctx, func(handlerCtx context.Context, message port.VideoProcessMessage) error {
		w.logger.Info("processing message", "video_id", message.VideoID)

		if err := w.useCase.Execute(handlerCtx, message); err != nil {
			w.logger.Error("message processing failed", "video_id", message.VideoID, "error", err)
			return err
		}

		w.logger.Info("message processed successfully", "video_id", message.VideoID)
		return nil
	})
}
