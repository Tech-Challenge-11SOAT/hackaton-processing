package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/thiagomartins/hackaton-processing/internal/domain"
	"github.com/thiagomartins/hackaton-processing/internal/port"
)

type fakeClock struct {
	current time.Time
}

func (f *fakeClock) Now() time.Time {
	t := f.current
	f.current = f.current.Add(1 * time.Second)
	return t
}

type inMemoryRepo struct {
	byVideoID map[string]*domain.ProcessingJob
}

func newInMemoryRepo() *inMemoryRepo {
	return &inMemoryRepo{byVideoID: map[string]*domain.ProcessingJob{}}
}

func (r *inMemoryRepo) Create(_ context.Context, job *domain.ProcessingJob) error {
	r.byVideoID[job.VideoID] = job
	return nil
}

func (r *inMemoryRepo) Update(_ context.Context, job *domain.ProcessingJob) error {
	r.byVideoID[job.VideoID] = job
	return nil
}

func (r *inMemoryRepo) FindByVideoID(_ context.Context, videoID string) (*domain.ProcessingJob, error) {
	if job, ok := r.byVideoID[videoID]; ok {
		return job, nil
	}
	return nil, nil
}

type fakeStorage struct {
	downloadErr error
	uploadErr   error
	uploads     []string
}

func (s *fakeStorage) Download(context.Context, string, string) error {
	return s.downloadErr
}

func (s *fakeStorage) Upload(_ context.Context, _ string, objectKey string) error {
	if s.uploadErr != nil {
		return s.uploadErr
	}
	s.uploads = append(s.uploads, objectKey)
	return nil
}

type fakeProcessor struct {
	frameCount int
	err        error
}

func (p *fakeProcessor) ExtractFramesToZip(context.Context, string, string) (int, error) {
	if p.err != nil {
		return 0, p.err
	}
	return p.frameCount, nil
}

type fakePublisher struct {
	statuses  []port.StatusUpdateMessage
	completed []port.VideoCompletedMessage
	failed    []port.VideoFailedMessage
}

func (p *fakePublisher) PublishStatusUpdate(_ context.Context, message port.StatusUpdateMessage) error {
	p.statuses = append(p.statuses, message)
	return nil
}

func (p *fakePublisher) PublishVideoCompleted(_ context.Context, message port.VideoCompletedMessage) error {
	p.completed = append(p.completed, message)
	return nil
}

func (p *fakePublisher) PublishVideoFailed(_ context.Context, message port.VideoFailedMessage) error {
	p.failed = append(p.failed, message)
	return nil
}

func TestProcessVideoUseCase_ExecuteSuccess(t *testing.T) {
	repo := newInMemoryRepo()
	storage := &fakeStorage{}
	processor := &fakeProcessor{frameCount: 12}
	publisher := &fakePublisher{}
	clock := &fakeClock{current: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)}

	uc := NewProcessVideoUseCase(repo, storage, processor, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)))
	uc.clock = clock

	message := port.VideoProcessMessage{
		VideoID:          "550e8400-e29b-41d4-a716-446655440000",
		UserID:           "user-1",
		S3VideoKey:       "videos/550e8400-e29b-41d4-a716-446655440000/original.mp4",
		OriginalFilename: "sample.mp4",
		CreatedAt:        clock.current,
	}

	if err := uc.Execute(context.Background(), message); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	job := repo.byVideoID[message.VideoID]
	if job == nil {
		t.Fatalf("expected job persisted")
	}
	if job.Status != domain.JobStatusCompleted {
		t.Fatalf("expected status %s, got %s", domain.JobStatusCompleted, job.Status)
	}

	if len(publisher.statuses) != 2 {
		t.Fatalf("expected 2 status events, got %d", len(publisher.statuses))
	}
	if publisher.statuses[0].Status != domain.JobStatusProcessing {
		t.Fatalf("expected first status event PROCESSING, got %s", publisher.statuses[0].Status)
	}
	if publisher.statuses[1].Status != domain.JobStatusCompleted {
		t.Fatalf("expected second status event COMPLETED, got %s", publisher.statuses[1].Status)
	}
	if len(publisher.completed) != 1 {
		t.Fatalf("expected 1 completed event, got %d", len(publisher.completed))
	}
	if len(publisher.failed) != 0 {
		t.Fatalf("expected no failed events, got %d", len(publisher.failed))
	}
}

func TestProcessVideoUseCase_ExecuteMarksFailedOnDownloadError(t *testing.T) {
	repo := newInMemoryRepo()
	storage := &fakeStorage{downloadErr: errors.New("s3 unavailable")}
	processor := &fakeProcessor{frameCount: 10}
	publisher := &fakePublisher{}
	clock := &fakeClock{current: time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)}

	uc := NewProcessVideoUseCase(repo, storage, processor, publisher, slog.New(slog.NewTextHandler(io.Discard, nil)))
	uc.clock = clock

	message := port.VideoProcessMessage{
		VideoID:          "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		UserID:           "user-1",
		S3VideoKey:       "videos/a1b2c3d4-e5f6-7890-abcd-ef1234567890/original.mp4",
		OriginalFilename: "sample.mp4",
	}

	err := uc.Execute(context.Background(), message)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "download input video") {
		t.Fatalf("expected download error, got: %v", err)
	}

	job := repo.byVideoID[message.VideoID]
	if job == nil {
		t.Fatalf("expected job persisted")
	}
	if job.Status != domain.JobStatusFailed {
		t.Fatalf("expected status %s, got %s", domain.JobStatusFailed, job.Status)
	}

	if len(publisher.statuses) != 2 {
		t.Fatalf("expected 2 status events, got %d", len(publisher.statuses))
	}
	if publisher.statuses[1].Status != domain.JobStatusFailed {
		t.Fatalf("expected failed status event, got %s", publisher.statuses[1].Status)
	}
	if len(publisher.failed) != 1 {
		t.Fatalf("expected 1 failed event, got %d", len(publisher.failed))
	}
	if len(publisher.completed) != 0 {
		t.Fatalf("expected no completed events, got %d", len(publisher.completed))
	}
}

func TestProcessVideoUseCase_ExecuteIgnoresTerminalDuplicate(t *testing.T) {
	repo := newInMemoryRepo()
	now := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)
	job, err := domain.NewProcessingJob("job-id", "video-dup", "videos/video-dup/original.mp4", now)
	if err != nil {
		t.Fatalf("unexpected create job error: %v", err)
	}
	if err := job.MarkProcessing(now.Add(1 * time.Second)); err != nil {
		t.Fatalf("unexpected mark processing error: %v", err)
	}
	if err := job.MarkCompleted("videos/video-dup/frames.zip", 5, now.Add(2*time.Second)); err != nil {
		t.Fatalf("unexpected mark completed error: %v", err)
	}
	repo.byVideoID["video-dup"] = job

	uc := NewProcessVideoUseCase(
		repo,
		&fakeStorage{},
		&fakeProcessor{frameCount: 5},
		&fakePublisher{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	msg := port.VideoProcessMessage{
		VideoID:    "video-dup",
		S3VideoKey: "videos/video-dup/original.mp4",
	}

	if err := uc.Execute(context.Background(), msg); err != nil {
		t.Fatalf("expected duplicate terminal to be ignored, got error: %v", err)
	}
}

func TestOutputZipKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantKey string
	}{
		{"normal key", "videos/123/original.mp4", "videos/123/frames.zip"},
		{"empty key", "", "frames.zip"},
		{"root file", "original.mp4", "frames.zip"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := outputZipKey(tt.input)
			if got != tt.wantKey {
				t.Fatalf("outputZipKey(%q) = %q, want %q", tt.input, got, tt.wantKey)
			}
		})
	}
}

func TestCreateWorkDirError(t *testing.T) {
	prev := osMkdirTemp
	defer func() { osMkdirTemp = prev }()

	osMkdirTemp = func(string, string) (string, error) {
		return "", fmt.Errorf("boom")
	}

	_, err := createWorkDir()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
