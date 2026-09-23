package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
	logger *slog.Logger
}

func NewConsumer(brokers []string, topic string, groupID string, log *slog.Logger) *Consumer {
	reader := kafka.NewReader(
		kafka.ReaderConfig{
			Brokers:           brokers,
			Topic:             topic,
			GroupID:           groupID,
			GroupBalancers:    []kafka.GroupBalancer{&kafka.RoundRobinGroupBalancer{}},
			MinBytes:          1,
			MaxBytes:          10e6,
			MaxWait:           1 * time.Second,
			CommitInterval:    0,
			StartOffset:       kafka.FirstOffset,
			RebalanceTimeout:  30 * time.Second,
			HeartbeatInterval: 3 * time.Second,
			Dialer: &kafka.Dialer{
				Timeout:   15 * time.Second,
				DualStack: true,
				KeepAlive: 30 * time.Second,
			},
		},
	)

	return &Consumer{
		reader: reader,
		logger: log,
	}
}

func (c *Consumer) Fetch(ctx context.Context) (kafka.Message, error) {
	message, err := c.reader.FetchMessage(ctx)
	if err != nil {
		c.logger.ErrorContext(ctx, "failed to fetch kafka message", "error", err)
		return kafka.Message{}, fmt.Errorf("fetch kafka message: %w", err)
	}
	return message, nil
}

func (c *Consumer) Commit(ctx context.Context, messages ...kafka.Message) error {
	if err := c.reader.CommitMessages(ctx, messages...); err != nil {
		c.logger.ErrorContext(ctx, "failed to commit kafka offsets", "error", err, "count", len(messages))
		return fmt.Errorf("commit kafka offset: %w", err)
	}

	if len(messages) == 1 {
		c.logger.InfoContext(ctx, "kafka offset committed successfully",
			"topic", messages[0].Topic,
			"partition", messages[0].Partition,
			"offset", messages[0].Offset,
		)
	} else {
		c.logger.InfoContext(ctx, "kafka offsets committed successfully", "count", len(messages))
	}
	return nil
}

func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		c.logger.Error("failed to close kafka reader", "error", err)
		return fmt.Errorf("close kafka reader: %w", err)
	}
	c.logger.Info("kafka reader closed successfully")
	return nil
}
