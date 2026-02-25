package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thiagomartins/hackaton-processing/internal/domain"
)

// Repository persists processing jobs in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a PostgreSQL-backed job repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Create inserts a new processing job row.
func (r *Repository) Create(ctx context.Context, job *domain.ProcessingJob) error {
	if job == nil {
		return errors.New("job is required")
	}

	const query = `
		INSERT INTO processing_jobs (
			id,
			video_id,
			s3_video_key,
			s3_zip_key,
			status,
			frame_count,
			error_message,
			started_at,
			completed_at,
			created_at,
			updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`

	_, err := r.pool.Exec(
		ctx,
		query,
		job.ID,
		job.VideoID,
		job.S3VideoKey,
		nullIfEmpty(job.S3ZipKey),
		string(job.Status),
		job.FrameCount,
		nullIfEmpty(job.ErrorMessage),
		job.StartedAt,
		job.CompletedAt,
		job.CreatedAt,
		job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert processing job: %w", err)
	}

	return nil
}

// Update updates mutable fields of a processing job by id.
func (r *Repository) Update(ctx context.Context, job *domain.ProcessingJob) error {
	if job == nil {
		return errors.New("job is required")
	}

	const query = `
		UPDATE processing_jobs
		SET
			s3_zip_key = $2,
			status = $3,
			frame_count = $4,
			error_message = $5,
			started_at = $6,
			completed_at = $7,
			updated_at = $8
		WHERE id = $1
	`

	tag, err := r.pool.Exec(
		ctx,
		query,
		job.ID,
		nullIfEmpty(job.S3ZipKey),
		string(job.Status),
		job.FrameCount,
		nullIfEmpty(job.ErrorMessage),
		job.StartedAt,
		job.CompletedAt,
		job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update processing job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

// FindByVideoID returns a job by its video id.
func (r *Repository) FindByVideoID(ctx context.Context, videoID string) (*domain.ProcessingJob, error) {
	if strings.TrimSpace(videoID) == "" {
		return nil, errors.New("videoID is required")
	}

	const query = `
		SELECT
			id,
			video_id,
			s3_video_key,
			COALESCE(s3_zip_key, ''),
			status,
			frame_count,
			COALESCE(error_message, ''),
			started_at,
			completed_at,
			created_at,
			updated_at
		FROM processing_jobs
		WHERE video_id = $1
	`

	var (
		job       domain.ProcessingJob
		statusRaw string
		startedAt *time.Time
		doneAt    *time.Time
	)

	err := r.pool.QueryRow(ctx, query, videoID).Scan(
		&job.ID,
		&job.VideoID,
		&job.S3VideoKey,
		&job.S3ZipKey,
		&statusRaw,
		&job.FrameCount,
		&job.ErrorMessage,
		&startedAt,
		&doneAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find processing job by video_id: %w", err)
	}

	job.Status = domain.JobStatus(statusRaw)
	job.StartedAt = startedAt
	job.CompletedAt = doneAt

	return &job, nil
}

func nullIfEmpty(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}
