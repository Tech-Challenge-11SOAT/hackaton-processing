package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// JobStatus represents the lifecycle state of a video processing job.
type JobStatus string

const (
	// JobStatusPending indicates the job is waiting to be processed.
	JobStatusPending JobStatus = "PENDING"
	// JobStatusProcessing indicates the job is currently being processed.
	JobStatusProcessing JobStatus = "PROCESSING"
	// JobStatusCompleted indicates the job finished successfully.
	JobStatusCompleted JobStatus = "COMPLETED"
	// JobStatusFailed indicates the job finished with error.
	JobStatusFailed JobStatus = "FAILED"
)

var (
	// ErrInvalidStatusTransition is returned when moving to a forbidden state.
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	// ErrInvalidFrameCount is returned when a completed job has invalid frame count.
	ErrInvalidFrameCount = errors.New("invalid frame count")
	// ErrMissingZipKey is returned when completed job has no generated zip key.
	ErrMissingZipKey = errors.New("missing zip key")
	// ErrMissingErrorMessage is returned when failed job has no error detail.
	ErrMissingErrorMessage = errors.New("missing error message")
)

// ProcessingJob is the domain aggregate for processing_db.processing_jobs.
type ProcessingJob struct {
	ID           string
	VideoID      string
	S3VideoKey   string
	S3ZipKey     string
	Status       JobStatus
	FrameCount   int
	ErrorMessage string
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewProcessingJob creates a new job in PENDING state.
func NewProcessingJob(id, videoID, s3VideoKey string, now time.Time) (*ProcessingJob, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("id is required")
	}
	if strings.TrimSpace(videoID) == "" {
		return nil, errors.New("video_id is required")
	}
	if strings.TrimSpace(s3VideoKey) == "" {
		return nil, errors.New("s3_video_key is required")
	}

	return &ProcessingJob{
		ID:         id,
		VideoID:    videoID,
		S3VideoKey: s3VideoKey,
		Status:     JobStatusPending,
		CreatedAt:  now.UTC(),
		UpdatedAt:  now.UTC(),
	}, nil
}

// MarkProcessing transitions PENDING -> PROCESSING.
func (j *ProcessingJob) MarkProcessing(startedAt time.Time) error {
	if j.Status != JobStatusPending {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidStatusTransition, j.Status, JobStatusProcessing)
	}

	start := startedAt.UTC()
	j.Status = JobStatusProcessing
	j.StartedAt = &start
	j.UpdatedAt = start
	j.ErrorMessage = ""

	return nil
}

// MarkCompleted transitions PROCESSING -> COMPLETED.
func (j *ProcessingJob) MarkCompleted(s3ZipKey string, frameCount int, completedAt time.Time) error {
	if j.Status != JobStatusProcessing {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidStatusTransition, j.Status, JobStatusCompleted)
	}
	if strings.TrimSpace(s3ZipKey) == "" {
		return ErrMissingZipKey
	}
	if frameCount <= 0 {
		return ErrInvalidFrameCount
	}

	doneAt := completedAt.UTC()
	j.Status = JobStatusCompleted
	j.S3ZipKey = s3ZipKey
	j.FrameCount = frameCount
	j.CompletedAt = &doneAt
	j.UpdatedAt = doneAt
	j.ErrorMessage = ""

	return nil
}

// MarkFailed transitions PROCESSING -> FAILED.
func (j *ProcessingJob) MarkFailed(errMessage string, failedAt time.Time) error {
	if j.Status != JobStatusProcessing {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidStatusTransition, j.Status, JobStatusFailed)
	}
	if strings.TrimSpace(errMessage) == "" {
		return ErrMissingErrorMessage
	}

	doneAt := failedAt.UTC()
	j.Status = JobStatusFailed
	j.ErrorMessage = errMessage
	j.CompletedAt = &doneAt
	j.UpdatedAt = doneAt

	return nil
}

// IsTerminal returns true if the status can no longer transition.
func (s JobStatus) IsTerminal() bool {
	return s == JobStatusCompleted || s == JobStatusFailed
}
