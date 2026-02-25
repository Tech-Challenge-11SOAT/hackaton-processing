package port

import (
	"context"
	"time"

	"github.com/thiagomartins/hackaton-processing/internal/domain"
)

// VideoProcessMessage represents an input event consumed from video.process.
type VideoProcessMessage struct {
	VideoID          string
	UserID           string
	S3VideoKey       string
	OriginalFilename string
	CreatedAt        time.Time
}

// StatusUpdateMessage represents an output event for status.update.
type StatusUpdateMessage struct {
	VideoID      string
	Status       domain.JobStatus
	S3ZipKey     string
	ErrorMessage string
	UpdatedAt    time.Time
}

// VideoCompletedMessage represents an output event for video.completed.
type VideoCompletedMessage struct {
	VideoID          string
	UserID           string
	OriginalFilename string
	S3ZipKey         string
	CompletedAt      time.Time
}

// VideoFailedMessage represents an output event for video.failed.
type VideoFailedMessage struct {
	VideoID          string
	UserID           string
	OriginalFilename string
	ErrorMessage     string
	FailedAt         time.Time
}

// ProcessingJobRepository provides persistence operations for processing jobs.
type ProcessingJobRepository interface {
	Create(ctx context.Context, job *domain.ProcessingJob) error
	Update(ctx context.Context, job *domain.ProcessingJob) error
	FindByVideoID(ctx context.Context, videoID string) (*domain.ProcessingJob, error)
}

// VideoProcessConsumer receives video.process events from the broker.
type VideoProcessConsumer interface {
	ConsumeVideoProcess(ctx context.Context, handler func(context.Context, VideoProcessMessage) error) error
}

// EventPublisher publishes domain events to the message broker.
type EventPublisher interface {
	PublishStatusUpdate(ctx context.Context, message StatusUpdateMessage) error
	PublishVideoCompleted(ctx context.Context, message VideoCompletedMessage) error
	PublishVideoFailed(ctx context.Context, message VideoFailedMessage) error
}

// ObjectStorage abstracts file download/upload operations.
type ObjectStorage interface {
	Download(ctx context.Context, objectKey, destinationPath string) error
	Upload(ctx context.Context, sourcePath, objectKey string) error
}

// VideoProcessor extracts all frames and stores them as a zip archive.
type VideoProcessor interface {
	ExtractFramesToZip(ctx context.Context, inputVideoPath, outputZipPath string) (int, error)
}
