package s3

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// Storage implements object operations backed by Amazon S3.
type Storage struct {
	bucket     string
	client     *awss3.Client
	downloader *manager.Downloader
	uploader   *manager.Uploader
}

// NewStorage creates a new S3 object storage adapter.
func NewStorage(bucket string, client *awss3.Client) (*Storage, error) {
	if strings.TrimSpace(bucket) == "" {
		return nil, errors.New("bucket is required")
	}
	if client == nil {
		return nil, errors.New("s3 client is required")
	}

	return &Storage{
		bucket:     bucket,
		client:     client,
		downloader: manager.NewDownloader(client),
		uploader:   manager.NewUploader(client),
	}, nil
}

// Download fetches an object from S3 and writes it to destinationPath.
func (s *Storage) Download(ctx context.Context, objectKey, destinationPath string) error {
	if strings.TrimSpace(objectKey) == "" {
		return errors.New("objectKey is required")
	}
	if strings.TrimSpace(destinationPath) == "" {
		return errors.New("destinationPath is required")
	}

	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}

	file, err := os.Create(destinationPath)
	if err != nil {
		return fmt.Errorf("create destination file: %w", err)
	}
	defer func() { _ = file.Close() }()

	_, err = s.downloader.Download(ctx, file, &awss3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	})
	if err != nil {
		return fmt.Errorf("download object from s3: %w", err)
	}

	return nil
}

// Upload sends a local file to S3 using objectKey as destination path.
func (s *Storage) Upload(ctx context.Context, sourcePath, objectKey string) error {
	if strings.TrimSpace(sourcePath) == "" {
		return errors.New("sourcePath is required")
	}
	if strings.TrimSpace(objectKey) == "" {
		return errors.New("objectKey is required")
	}

	file, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer func() { _ = file.Close() }()

	_, err = s.uploader.Upload(ctx, &awss3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("upload object to s3: %w", err)
	}

	return nil
}
