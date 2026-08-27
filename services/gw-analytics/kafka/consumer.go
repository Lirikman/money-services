package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/Lirikman/money_services/services/gw-analytics/models"
	kafkaGo "github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader  *kafkaGo.Reader
	service *service.AnalyticsService
	logger  *slog.Logger
}

var ErrInvalidTransactionID = errors.New(
	"transaction_id is required",
)

func NewConsumer(brokers []string, topic string, groupID string, svc *service.AnalyticsService, log *slog.Logger) *Consumer {
	reader := kafkaGo.NewReader(kafkaGo.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: groupID,

		MinBytes: 1,
		MaxBytes: 10e6,

		CommitInterval: 0,
	})

	return &Consumer{
		reader:  reader,
		service: svc,
		logger:  log,
	}
}

// Чтение сообщений из kafka
func (c *Consumer) Run(ctx context.Context) error {

	for {
		message, err := c.reader.FetchMessage(ctx)

		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			c.logger.Error("failed to fetch kafka message", slog.Any("error", err))

			continue
		}

		if err := c.handleMessage(ctx, message); err != nil {

			c.logger.Error("failed to process kafka message",
				slog.Any("error", err),
				slog.Int("partition", message.Partition),
				slog.Int64("offset", message.Offset),
			)

			// offset НЕ коммитим.
			// Kafka доставит сообщение повторно.
			continue
		}

		if err := c.reader.CommitMessages(ctx, message); err != nil {
			c.logger.Error("failed to commit kafka offset",
				slog.Any("error", err),
			)

			continue
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, message kafkaGo.Message) error {
	var event models.TransactionEvent

	if err := json.Unmarshal(message.Value, &event); err != nil {
		return err
	}

	if event.Status == "" {
		event.Status = "received"
	}

	if event.TransactionID == "" {
		return ErrInvalidTransactionID
	}

	receivedAt := time.Now().UTC()

	return c.service.Process(ctx, event, receivedAt)
}
