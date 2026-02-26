package rabbitmq

import (
	"testing"
	"time"
)

func TestDecodeVideoProcessMessage_AcceptsCreatedAtWithoutTimezone(t *testing.T) {
	body := []byte(`{
		"videoId":"4cad53f8-9ee7-47e6-bef9-02bb21340477",
		"userId":"2a775693-2899-4640-b7f1-420b2f946248",
		"s3VideoKey":"4cad53f8-9ee7-47e6-bef9-02bb21340477/videoteste.mov",
		"originalFileName":"videoteste.mov",
		"createdAt":"2026-02-25T21:55:23.2150773"
	}`)

	message, err := decodeVideoProcessMessage(body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedCreatedAt := time.Date(2026, 2, 25, 21, 55, 23, 215077300, time.UTC)
	if !message.CreatedAt.Equal(expectedCreatedAt) {
		t.Fatalf("expected createdAt %v, got %v", expectedCreatedAt, message.CreatedAt)
	}

	if message.OriginalFilename != "videoteste.mov" {
		t.Fatalf("expected originalFilename videoteste.mov, got %q", message.OriginalFilename)
	}
}

func TestDecodeVideoProcessMessage_AcceptsRFC3339CreatedAt(t *testing.T) {
	body := []byte(`{
		"videoId":"550e8400-e29b-41d4-a716-446655440000",
		"userId":"7ad7f47d-0bce-4c0d-a7ff-f35b7f5d67f1",
		"s3VideoKey":"videos/550e8400-e29b-41d4-a716-446655440000/original.mp4",
		"originalFilename":"seu-video.mp4",
		"createdAt":"2026-02-24T10:00:00Z"
	}`)

	message, err := decodeVideoProcessMessage(body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedCreatedAt := time.Date(2026, 2, 24, 10, 0, 0, 0, time.UTC)
	if !message.CreatedAt.Equal(expectedCreatedAt) {
		t.Fatalf("expected createdAt %v, got %v", expectedCreatedAt, message.CreatedAt)
	}
}
