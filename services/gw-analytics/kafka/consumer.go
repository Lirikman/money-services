package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Lirikman/money_services/services/gw-analytics/service"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
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
		MaxWait:  1 * time.Second,

		CommitInterval:    0,
		RebalanceTimeout:  30 * time.Second,
		HeartbeatInterval: 3 * time.Second,
		Dialer: &kafkaGo.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
		},
	})

	return &Consumer{
		reader:  reader,
		service: svc,
		logger:  log,
	}
}

const (
	batchSize    = 1000
	flushTimeout = 3 * time.Second
)

func (c *Consumer) readMessages(ctx context.Context, out chan<- kafkaGo.Message) error {
	defer close(out)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			c.logger.Error("Fetch kafka message", slog.Any("error", err))
			return err
		}

		select {
		case out <- msg:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	c.logger.Info("KAFKA CONSUMER STARTED")

	messages := make(chan kafkaGo.Message, 1000)

	go func() {
		if err := c.readMessages(ctx, messages); err != nil {
			c.logger.Error("kafka reader stopped", slog.Any("error", err))
		}
	}()

	batch := make([]kafkaGo.Message, 0, batchSize)

	timer := time.NewTimer(flushTimeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				_ = c.processBatch(context.Background(), batch)
			}
			return nil

		case msg, ok := <-messages:
			if !ok {
				return nil
			}
			batch = append(batch, msg)
			if len(batch) >= batchSize {
				if err := c.processBatch(ctx, batch); err != nil {
					return err
				}
				batch = batch[:0]
				resetTimer(timer, flushTimeout)
			}

		case <-timer.C:
			if len(batch) > 0 {
				if err := c.processBatch(ctx, batch); err != nil {
					return err
				}
				batch = batch[:0]
			}
			resetTimer(timer, flushTimeout)
		}
	}
}

func (c *Consumer) processBatch(ctx context.Context, messages []kafkaGo.Message) error {
	if len(messages) == 0 {
		return nil
	}

	events := make([]models.TransactionEvent, 0, len(messages))
	validMessages := make([]kafkaGo.Message, 0, len(messages))

	for _, msg := range messages {
		event, err := c.parseMessage(msg)

		if err != nil {
			c.logger.Error("invalid kafka message",
				slog.Any("error", err),
				slog.Int64("offset", msg.Offset),
			)
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				return fmt.Errorf("commit poison message: %w", err)
			}
			continue
		}
		events = append(events, event)
		validMessages = append(validMessages, msg)
	}

	if len(events) == 0 {
		return nil
	}

	receivedAt := time.Now().UTC()

	if err := c.service.ProcessBatch(ctx, events, receivedAt); err != nil {
		c.logger.Error("clickhouse batch processing failed",
			slog.Any("error", err),
			slog.Int("batch_size", len(events)),
		)
		return err
	}

	if err := c.reader.CommitMessages(ctx, validMessages...); err != nil {
		return fmt.Errorf("commit kafka batch: %w", err)
	}

	c.logger.Info("batch processed successfully", slog.Int("batch_size", len(events)))
	return nil
}

func (c *Consumer) parseMessage(msg kafkaGo.Message) (models.TransactionEvent, error) {
	var event models.TransactionEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return event, err
	}
	if event.TransactionID == "" {
		return event, errors.New("empty transaction_id")
	}
	if event.Status == "" {
		event.Status = "received"
	}
	return event, nil
}

func resetTimer(timer *time.Timer, duration time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(duration)
}
