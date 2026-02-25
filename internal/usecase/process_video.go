package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thiagomartins/hackaton-processing/internal/domain"
	"github.com/thiagomartins/hackaton-processing/internal/port"
)

// Clock abstracts time for deterministic tests.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now().UTC()
}

// ProcessVideoUseCase orchestrates end-to-end processing for one video message.
type ProcessVideoUseCase struct {
	repository    port.ProcessingJobRepository
	inputStorage  port.ObjectStorage
	outputStorage port.ObjectStorage
	processor     port.VideoProcessor
	publisher     port.EventPublisher
	logger        *slog.Logger
	clock         Clock
}

// NewProcessVideoUseCase creates the use case for video processing orchestration.
func NewProcessVideoUseCase(
	repository port.ProcessingJobRepository,
	inputStorage port.ObjectStorage,
	outputStorage port.ObjectStorage,
	processor port.VideoProcessor,
	publisher port.EventPublisher,
	logger *slog.Logger,
) *ProcessVideoUseCase {
	if logger == nil {
		logger = slog.Default()
	}

	return &ProcessVideoUseCase{
		repository:    repository,
		inputStorage:  inputStorage,
		outputStorage: outputStorage,
		processor:     processor,
		publisher:     publisher,
		logger:        logger,
		clock:         realClock{},
	}
}

// Execute processes one video.process event.
func (u *ProcessVideoUseCase) Execute(ctx context.Context, message port.VideoProcessMessage) error {
	now := u.clock.Now()

	job, err := u.repository.FindByVideoID(ctx, message.VideoID)
	if err != nil {
		return fmt.Errorf("find existing job: %w", err)
	}

	if job != nil && job.Status.IsTerminal() {
		u.logger.Info("ignoring duplicate terminal message",
			"video_id", message.VideoID,
			"status", job.Status,
		)
		return nil
	}

	if job == nil {
		job, err = domain.NewProcessingJob(message.VideoID, message.VideoID, message.S3VideoKey, now)
		if err != nil {
			return fmt.Errorf("create processing job entity: %w", err)
		}

		if err := u.repository.Create(ctx, job); err != nil {
			return fmt.Errorf("persist new processing job: %w", err)
		}
	}

	if job.Status == domain.JobStatusProcessing {
		u.logger.Info("job already processing, skipping duplicate in-flight message", "video_id", message.VideoID)
		return nil
	}

	if err := job.MarkProcessing(now); err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}
	if err := u.repository.Update(ctx, job); err != nil {
		return fmt.Errorf("persist processing status: %w", err)
	}
	if err := u.publishStatus(ctx, message.VideoID, domain.JobStatusProcessing, "", "", u.clock.Now()); err != nil {
		return fmt.Errorf("publish processing status: %w", err)
	}

	workDir, err := createWorkDir()
	if err != nil {
		_ = u.failJob(ctx, job, message, fmt.Sprintf("create workdir: %v", err))
		return fmt.Errorf("create workdir: %w", err)
	}
	defer removeDir(workDir)

	inputPath := filepath.Join(workDir, "input-video")
	zipPath := filepath.Join(workDir, "frames.zip")

	if err := u.inputStorage.Download(ctx, message.S3VideoKey, inputPath); err != nil {
		_ = u.failJob(ctx, job, message, fmt.Sprintf("download input video: %v", err))
		return fmt.Errorf("download input video: %w", err)
	}

	frameCount, err := u.processor.ExtractFramesToZip(ctx, inputPath, zipPath)
	if err != nil {
		_ = u.failJob(ctx, job, message, fmt.Sprintf("extract frames: %v", err))
		return fmt.Errorf("extract frames: %w", err)
	}

	zipKey := outputZipKey(message.VideoID)
	if err := u.outputStorage.Upload(ctx, zipPath, zipKey); err != nil {
		_ = u.failJob(ctx, job, message, fmt.Sprintf("upload zip: %v", err))
		return fmt.Errorf("upload zip: %w", err)
	}

	doneAt := u.clock.Now()
	if err := job.MarkCompleted(zipKey, frameCount, doneAt); err != nil {
		return fmt.Errorf("mark completed: %w", err)
	}
	if err := u.repository.Update(ctx, job); err != nil {
		return fmt.Errorf("persist completed status: %w", err)
	}
	if err := u.publishStatus(ctx, message.VideoID, domain.JobStatusCompleted, zipKey, "", doneAt); err != nil {
		return fmt.Errorf("publish completed status: %w", err)
	}
	if err := u.publisher.PublishVideoCompleted(ctx, port.VideoCompletedMessage{
		VideoID:          message.VideoID,
		UserID:           message.UserID,
		OriginalFilename: message.OriginalFilename,
		S3ZipKey:         zipKey,
		CompletedAt:      doneAt,
	}); err != nil {
		return fmt.Errorf("publish video.completed: %w", err)
	}

	return nil
}

func (u *ProcessVideoUseCase) failJob(
	ctx context.Context,
	job *domain.ProcessingJob,
	message port.VideoProcessMessage,
	errMessage string,
) error {
	failedAt := u.clock.Now()

	if markErr := job.MarkFailed(errMessage, failedAt); markErr != nil {
		return fmt.Errorf("mark failed: %w", markErr)
	}
	if err := u.repository.Update(ctx, job); err != nil {
		return fmt.Errorf("persist failed status: %w", err)
	}
	if err := u.publishStatus(ctx, message.VideoID, domain.JobStatusFailed, "", errMessage, failedAt); err != nil {
		return fmt.Errorf("publish failed status: %w", err)
	}
	if err := u.publisher.PublishVideoFailed(ctx, port.VideoFailedMessage{
		VideoID:          message.VideoID,
		UserID:           message.UserID,
		OriginalFilename: message.OriginalFilename,
		ErrorMessage:     errMessage,
		FailedAt:         failedAt,
	}); err != nil {
		return fmt.Errorf("publish video.failed: %w", err)
	}

	return nil
}

func (u *ProcessVideoUseCase) publishStatus(
	ctx context.Context,
	videoID string,
	status domain.JobStatus,
	s3ZipKey string,
	errorMessage string,
	updatedAt time.Time,
) error {
	return u.publisher.PublishStatusUpdate(ctx, port.StatusUpdateMessage{
		VideoID:      videoID,
		Status:       status,
		S3ZipKey:     s3ZipKey,
		ErrorMessage: errorMessage,
		UpdatedAt:    updatedAt,
	})
}

func outputZipKey(videoID string) string {
	cleaned := strings.TrimSpace(videoID)
	if cleaned == "" {
		return "frames.zip"
	}
	return filepath.ToSlash(filepath.Join(cleaned, "frames.zip"))
}

func createWorkDir() (string, error) {
	return osMkdirTemp("", "processing-job-*")
}

func removeDir(dir string) {
	_ = osRemoveAll(dir)
}

var (
	osMkdirTemp = os.MkdirTemp
	osRemoveAll = os.RemoveAll
)
