package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/thiagomartins/hackaton-processing/internal/port"
)

const (
	defaultProcessQueue   = "video.process"
	defaultStatusQueue    = "status.update"
	defaultCompletedQueue = "video.completed"
	defaultFailedQueue    = "video.failed"
)

// Config configures queue names and consumer behavior.
type Config struct {
	VideoProcessQueue string
	StatusUpdateQueue string
	VideoCompletedQ   string
	VideoFailedQ      string
	ConsumerTag       string
	PrefetchCount     int
}

// Adapter provides RabbitMQ implementations for consumer and publisher ports.
type Adapter struct {
	connection *amqp.Connection
	channel    *amqp.Channel
	config     Config
}

// NewAdapter creates a new RabbitMQ adapter with configured queues.
func NewAdapter(connection *amqp.Connection, cfg Config) (*Adapter, error) {
	if connection == nil {
		return nil, errors.New("connection is required")
	}

	ch, err := connection.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel: %w", err)
	}

	cfg = withDefaults(cfg)
	if cfg.PrefetchCount > 0 {
		if err := ch.Qos(cfg.PrefetchCount, 0, false); err != nil {
			_ = ch.Close()
			return nil, fmt.Errorf("set qos: %w", err)
		}
	}

	for _, q := range []string{
		cfg.VideoProcessQueue,
		cfg.StatusUpdateQueue,
		cfg.VideoCompletedQ,
		cfg.VideoFailedQ,
	} {
		if _, err := ch.QueueDeclare(q, true, false, false, false, nil); err != nil {
			_ = ch.Close()
			return nil, fmt.Errorf("declare queue %s: %w", q, err)
		}
	}

	return &Adapter{
		connection: connection,
		channel:    ch,
		config:     cfg,
	}, nil
}

// Close releases channel resources.
func (a *Adapter) Close() error {
	if a.channel == nil {
		return nil
	}
	return a.channel.Close()
}

// ConsumeVideoProcess consumes messages from video.process and acks on success.
func (a *Adapter) ConsumeVideoProcess(
	ctx context.Context,
	handler func(context.Context, port.VideoProcessMessage) error,
) error {
	if handler == nil {
		return errors.New("handler is required")
	}

	deliveries, err := a.channel.Consume(
		a.config.VideoProcessQueue,
		a.config.ConsumerTag,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume queue %s: %w", a.config.VideoProcessQueue, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("rabbitmq deliveries channel closed")
			}

			message, err := decodeVideoProcessMessage(delivery.Body)
			if err != nil {
				slog.Default().Error(
					"failed to decode video.process message",
					"error", err,
					"consumer_tag", a.config.ConsumerTag,
					"queue", a.config.VideoProcessQueue,
					"payload", string(delivery.Body),
				)
				_ = delivery.Nack(false, false)
				continue
			}

			if err := handler(ctx, message); err != nil {
				_ = delivery.Nack(false, true)
				continue
			}

			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("ack delivery: %w", err)
			}
		}
	}
}

// PublishStatusUpdate publishes to status.update queue.
func (a *Adapter) PublishStatusUpdate(ctx context.Context, message port.StatusUpdateMessage) error {
	return a.publishJSON(ctx, a.config.StatusUpdateQueue, message)
}

// PublishVideoCompleted publishes to video.completed queue.
func (a *Adapter) PublishVideoCompleted(ctx context.Context, message port.VideoCompletedMessage) error {
	return a.publishJSON(ctx, a.config.VideoCompletedQ, message)
}

// PublishVideoFailed publishes to video.failed queue.
func (a *Adapter) PublishVideoFailed(ctx context.Context, message port.VideoFailedMessage) error {
	return a.publishJSON(ctx, a.config.VideoFailedQ, message)
}

func (a *Adapter) publishJSON(ctx context.Context, queue string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	if err := a.channel.PublishWithContext(
		ctx,
		"",
		queue,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now().UTC(),
		},
	); err != nil {
		return fmt.Errorf("publish to queue %s: %w", queue, err)
	}

	return nil
}

func withDefaults(cfg Config) Config {
	if strings.TrimSpace(cfg.VideoProcessQueue) == "" {
		cfg.VideoProcessQueue = defaultProcessQueue
	}
	if strings.TrimSpace(cfg.StatusUpdateQueue) == "" {
		cfg.StatusUpdateQueue = defaultStatusQueue
	}
	if strings.TrimSpace(cfg.VideoCompletedQ) == "" {
		cfg.VideoCompletedQ = defaultCompletedQueue
	}
	if strings.TrimSpace(cfg.VideoFailedQ) == "" {
		cfg.VideoFailedQ = defaultFailedQueue
	}
	if strings.TrimSpace(cfg.ConsumerTag) == "" {
		cfg.ConsumerTag = "processing-service-consumer"
	}
	return cfg
}

func decodeVideoProcessMessage(body []byte) (port.VideoProcessMessage, error) {
	var payload struct {
		VideoID           string `json:"videoId"`
		UserID            string `json:"userId"`
		S3VideoKey        string `json:"s3VideoKey"`
		OriginalFilename  string `json:"originalFilename"`
		OriginalFileName  string `json:"originalFileName"`
		CreatedAt         string `json:"createdAt"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return port.VideoProcessMessage{}, fmt.Errorf("unmarshal video.process message: %w", err)
	}

	if strings.TrimSpace(payload.VideoID) == "" {
		return port.VideoProcessMessage{}, errors.New("videoId is required")
	}
	if strings.TrimSpace(payload.S3VideoKey) == "" {
		return port.VideoProcessMessage{}, errors.New("s3VideoKey is required")
	}

	createdAt, err := parseCreatedAt(payload.CreatedAt)
	if err != nil {
		return port.VideoProcessMessage{}, err
	}

	originalFilename := payload.OriginalFilename
	if strings.TrimSpace(originalFilename) == "" {
		originalFilename = payload.OriginalFileName
	}

	return port.VideoProcessMessage{
		VideoID:          payload.VideoID,
		UserID:           payload.UserID,
		S3VideoKey:       payload.S3VideoKey,
		OriginalFilename: originalFilename,
		CreatedAt:        createdAt,
	}, nil
}

func parseCreatedAt(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, nil
	}

	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err == nil {
		return parsed, nil
	}

	parsed, err = time.ParseInLocation("2006-01-02T15:04:05.9999999", trimmed, time.UTC)
	if err == nil {
		return parsed, nil
	}

	return time.Time{}, fmt.Errorf("invalid createdAt, expected RFC3339 or yyyy-mm-ddThh:mm:ss.fffffff: %w", err)
}
