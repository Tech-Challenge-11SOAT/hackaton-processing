package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewProcessingJob(t *testing.T) {
	now := time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		id        string
		videoID   string
		s3Video   string
		assertErr bool
	}{
		{
			name:      "creates pending job with valid fields",
			id:        "job-1",
			videoID:   "video-1",
			s3Video:   "videos/video-1/original.mp4",
			assertErr: false,
		},
		{
			name:      "fails when id is empty",
			id:        "",
			videoID:   "video-1",
			s3Video:   "videos/video-1/original.mp4",
			assertErr: true,
		},
		{
			name:      "fails when video id is empty",
			id:        "job-1",
			videoID:   "",
			s3Video:   "videos/video-1/original.mp4",
			assertErr: true,
		},
		{
			name:      "fails when s3 video key is empty",
			id:        "job-1",
			videoID:   "video-1",
			s3Video:   "",
			assertErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			job, err := NewProcessingJob(tt.id, tt.videoID, tt.s3Video, now)
			if tt.assertErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if job.Status != JobStatusPending {
				t.Fatalf("expected status %s, got %s", JobStatusPending, job.Status)
			}
			if !job.CreatedAt.Equal(now) {
				t.Fatalf("expected created_at %v, got %v", now, job.CreatedAt)
			}
			if !job.UpdatedAt.Equal(now) {
				t.Fatalf("expected updated_at %v, got %v", now, job.UpdatedAt)
			}
		})
	}
}

func TestProcessingJobTransitions(t *testing.T) {
	createdAt := time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC)
	startedAt := createdAt.Add(2 * time.Minute)
	completedAt := startedAt.Add(3 * time.Minute)

	t.Run("pending to processing to completed", func(t *testing.T) {
		job, err := NewProcessingJob("job-1", "video-1", "videos/video-1/original.mp4", createdAt)
		if err != nil {
			t.Fatalf("unexpected error creating job: %v", err)
		}

		if err := job.MarkProcessing(startedAt); err != nil {
			t.Fatalf("unexpected error marking processing: %v", err)
		}

		if err := job.MarkCompleted("videos/video-1/frames.zip", 42, completedAt); err != nil {
			t.Fatalf("unexpected error marking completed: %v", err)
		}

		if job.Status != JobStatusCompleted {
			t.Fatalf("expected status %s, got %s", JobStatusCompleted, job.Status)
		}
		if !job.Status.IsTerminal() {
			t.Fatalf("expected completed to be terminal")
		}
		if job.FrameCount != 42 {
			t.Fatalf("expected frame_count 42, got %d", job.FrameCount)
		}
		if job.S3ZipKey != "videos/video-1/frames.zip" {
			t.Fatalf("unexpected s3_zip_key: %s", job.S3ZipKey)
		}
	})

	t.Run("processing to failed", func(t *testing.T) {
		job, err := NewProcessingJob("job-2", "video-2", "videos/video-2/original.mp4", createdAt)
		if err != nil {
			t.Fatalf("unexpected error creating job: %v", err)
		}

		if err := job.MarkProcessing(startedAt); err != nil {
			t.Fatalf("unexpected error marking processing: %v", err)
		}
		if err := job.MarkFailed("ffmpeg failed", completedAt); err != nil {
			t.Fatalf("unexpected error marking failed: %v", err)
		}

		if job.Status != JobStatusFailed {
			t.Fatalf("expected status %s, got %s", JobStatusFailed, job.Status)
		}
		if !job.Status.IsTerminal() {
			t.Fatalf("expected failed to be terminal")
		}
		if job.ErrorMessage == "" {
			t.Fatalf("expected error message to be set")
		}
	})

	t.Run("invalid transition pending to completed", func(t *testing.T) {
		job, err := NewProcessingJob("job-3", "video-3", "videos/video-3/original.mp4", createdAt)
		if err != nil {
			t.Fatalf("unexpected error creating job: %v", err)
		}

		err = job.MarkCompleted("videos/video-3/frames.zip", 10, completedAt)
		if !errors.Is(err, ErrInvalidStatusTransition) {
			t.Fatalf("expected ErrInvalidStatusTransition, got %v", err)
		}
	})

	t.Run("invalid completed payload", func(t *testing.T) {
		job, err := NewProcessingJob("job-4", "video-4", "videos/video-4/original.mp4", createdAt)
		if err != nil {
			t.Fatalf("unexpected error creating job: %v", err)
		}

		if err := job.MarkProcessing(startedAt); err != nil {
			t.Fatalf("unexpected error marking processing: %v", err)
		}

		err = job.MarkCompleted("", 1, completedAt)
		if !errors.Is(err, ErrMissingZipKey) {
			t.Fatalf("expected ErrMissingZipKey, got %v", err)
		}

		err = job.MarkCompleted("videos/video-4/frames.zip", 0, completedAt)
		if !errors.Is(err, ErrInvalidFrameCount) {
			t.Fatalf("expected ErrInvalidFrameCount, got %v", err)
		}
	})

	t.Run("invalid failed payload", func(t *testing.T) {
		job, err := NewProcessingJob("job-5", "video-5", "videos/video-5/original.mp4", createdAt)
		if err != nil {
			t.Fatalf("unexpected error creating job: %v", err)
		}

		if err := job.MarkProcessing(startedAt); err != nil {
			t.Fatalf("unexpected error marking processing: %v", err)
		}

		err = job.MarkFailed(" ", completedAt)
		if !errors.Is(err, ErrMissingErrorMessage) {
			t.Fatalf("expected ErrMissingErrorMessage, got %v", err)
		}
	})
}
