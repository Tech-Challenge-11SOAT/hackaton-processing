package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/thiagomartins/hackaton-processing/internal/port"
)

type fakeConsumer struct {
	message     port.VideoProcessMessage
	consumeErr  error
	handlerErr  error
	handlerUsed bool
}

func (c *fakeConsumer) ConsumeVideoProcess(
	ctx context.Context,
	handler func(context.Context, port.VideoProcessMessage) error,
) error {
	if c.consumeErr != nil {
		return c.consumeErr
	}
	c.handlerUsed = true
	err := handler(ctx, c.message)
	if c.handlerErr != nil {
		return c.handlerErr
	}
	return err
}

type fakeExecutor struct {
	err error
}

func (e *fakeExecutor) Execute(context.Context, port.VideoProcessMessage) error {
	return e.err
}

func TestVideoProcessWorker_Run(t *testing.T) {
	t.Run("returns error when consumer is nil", func(t *testing.T) {
		w := NewVideoProcessWorker(nil, &fakeExecutor{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
		err := w.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("returns error when use case is nil", func(t *testing.T) {
		w := NewVideoProcessWorker(&fakeConsumer{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
		err := w.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("processes message successfully", func(t *testing.T) {
		consumer := &fakeConsumer{
			message: port.VideoProcessMessage{VideoID: "video-1"},
		}
		exec := &fakeExecutor{}
		w := NewVideoProcessWorker(consumer, exec, slog.New(slog.NewTextHandler(io.Discard, nil)))

		if err := w.Run(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !consumer.handlerUsed {
			t.Fatalf("expected handler to be used")
		}
	})

	t.Run("returns processing error", func(t *testing.T) {
		consumer := &fakeConsumer{
			message: port.VideoProcessMessage{VideoID: "video-2"},
		}
		exec := &fakeExecutor{err: errors.New("process failed")}
		w := NewVideoProcessWorker(consumer, exec, slog.New(slog.NewTextHandler(io.Discard, nil)))

		err := w.Run(context.Background())
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
