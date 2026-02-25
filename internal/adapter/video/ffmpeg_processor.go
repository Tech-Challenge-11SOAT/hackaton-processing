package video

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// FFmpegProcessor extracts all video frames and writes them into a zip file.
type FFmpegProcessor struct {
	ffmpegBinary string
}

// NewFFmpegProcessor creates a processor using the provided ffmpeg binary path.
func NewFFmpegProcessor(ffmpegBinary string) (*FFmpegProcessor, error) {
	if strings.TrimSpace(ffmpegBinary) == "" {
		ffmpegBinary = "ffmpeg"
	}

	return &FFmpegProcessor{ffmpegBinary: ffmpegBinary}, nil
}

// ExtractFramesToZip extracts all frames from inputVideoPath into outputZipPath.
func (p *FFmpegProcessor) ExtractFramesToZip(
	ctx context.Context,
	inputVideoPath, outputZipPath string,
) (int, error) {
	if strings.TrimSpace(inputVideoPath) == "" {
		return 0, errors.New("inputVideoPath is required")
	}
	if strings.TrimSpace(outputZipPath) == "" {
		return 0, errors.New("outputZipPath is required")
	}

	if err := os.MkdirAll(filepath.Dir(outputZipPath), 0o755); err != nil {
		return 0, fmt.Errorf("create output directory: %w", err)
	}

	framesDir, err := os.MkdirTemp("", "processing-frames-*")
	if err != nil {
		return 0, fmt.Errorf("create frames temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(framesDir) }()

	framePattern := filepath.Join(framesDir, "frame_%08d.jpg")
	cmd := exec.CommandContext(
		ctx,
		p.ffmpegBinary,
		"-y",
		"-i", inputVideoPath,
		framePattern,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("run ffmpeg: %w: %s", err, string(output))
	}

	framePaths, err := listFiles(framesDir)
	if err != nil {
		return 0, fmt.Errorf("list extracted frames: %w", err)
	}
	if len(framePaths) == 0 {
		return 0, errors.New("no frames extracted from input video")
	}

	if err := zipFiles(outputZipPath, framePaths); err != nil {
		return 0, fmt.Errorf("create frames zip: %w", err)
	}

	return len(framePaths), nil
}

func listFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}

	sort.Strings(paths)
	return paths, nil
}

func zipFiles(zipPath string, filePaths []string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer func() { _ = zipFile.Close() }()

	zipWriter := zip.NewWriter(zipFile)
	defer func() { _ = zipWriter.Close() }()

	for _, path := range filePaths {
		if err := addFileToZip(zipWriter, path); err != nil {
			return err
		}
	}

	return nil
}

func addFileToZip(zipWriter *zip.Writer, filePath string) error {
	src, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	writer, err := zipWriter.Create(filepath.Base(filePath))
	if err != nil {
		return err
	}

	if _, err := io.Copy(writer, src); err != nil {
		return err
	}

	return nil
}
