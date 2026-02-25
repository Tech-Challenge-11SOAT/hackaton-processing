CREATE TABLE IF NOT EXISTS processing_jobs (
    id UUID PRIMARY KEY,
    video_id UUID NOT NULL UNIQUE,
    s3_video_key VARCHAR(1024) NOT NULL,
    s3_zip_key VARCHAR(1024),
    status VARCHAR(20) NOT NULL CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED')),
    frame_count INTEGER NOT NULL DEFAULT 0 CHECK (frame_count >= 0),
    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_processing_jobs_status ON processing_jobs (status);
CREATE INDEX IF NOT EXISTS idx_processing_jobs_created_at ON processing_jobs (created_at DESC);
