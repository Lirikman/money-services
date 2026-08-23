package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader *kafka.Reader
}

// Создание нового консьюмера
// Ручной коммит прочитанных сообщений
func NewConsumer(brokers []string, topic string, groupID string) *Consumer {
	reader := kafka.NewReader(
		kafka.ReaderConfig{
			Brokers:        brokers,
			Topic:          topic,
			GroupID:        groupID,
			MinBytes:       1,
			MaxBytes:       10e6,
			MaxWait:        100 * time.Millisecond,
			CommitInterval: 0,
			StartOffset:    kafka.FirstOffset,
		},
	)

	return &Consumer{
		reader: reader,
	}
}

// Получение сообщения из kafka
func (c *Consumer) Fetch(ctx context.Context) (kafka.Message, error) {
	message, err := c.reader.FetchMessage(ctx)

	if err != nil {
		return kafka.Message{}, fmt.Errorf("fetch kafka message: %w", err)
	}

	return message, nil
}

// Коммит прочитанных сообщений
func (c *Consumer) Commit(ctx context.Context, messages ...kafka.Message) error {
	if err := c.reader.CommitMessages(ctx, messages...); err != nil {
		return fmt.Errorf("commit kafka offset: %w", err)
	}
	return nil
}

// Закрытие kafka.reader
func (c *Consumer) Close() error {
	return c.reader.Close()
}
