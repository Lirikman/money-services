package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Lirikman/money_services/services/gw-analytics/models"
	"github.com/Lirikman/money_services/services/gw-analytics/service"
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
		MaxWait:  100 * time.Millisecond,

		CommitInterval:    0,
		RebalanceTimeout:  30 * time.Second,
		HeartbeatInterval: 3 * time.Second,
		Dialer: &kafkaGo.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
		},
		Logger:      kafkaGo.LoggerFunc(func(msg string, args ...any) { log.Info(fmt.Sprintf("KAFKA INFO: "+msg, args...)) }),
		ErrorLogger: kafkaGo.LoggerFunc(func(msg string, args ...any) { log.Warn(fmt.Sprintf("KAFKA WARN: "+msg, args...)) }),
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

// Чтение сообщений из kafka
func (c *Consumer) Run(ctx context.Context) error {
	// Буфер для накопления сообщений из Kafka
	kafkaMessages := make([]kafkaGo.Message, 0, batchSize)
	// Буфер для распарсенных событий
	events := make([]models.TransactionEvent, 0, batchSize)

	ticker := time.NewTicker(flushTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Перед выходом записываем остатки данных
			if len(events) > 0 {
				_ = c.flushBatch(context.Background(), events, kafkaMessages)
			}
			return ctx.Err()
		case <-ticker.C:
			// Сработал таймаут — отправляем то, что успели накопить
			if len(events) > 0 {
				if err := c.flushBatch(ctx, events, kafkaMessages); err != nil {
					c.logger.Error("failed to flush batch by ticker", slog.Any("error", err))
				}
				// Очищаем буферы
				events = events[:0]
				kafkaMessages = kafkaMessages[:0]
			}
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				c.logger.Error("failed to fetch message", slog.Any("error", err))
				continue
			}
			// Парсим сообщение
			event, err := c.parseMessage(msg)
			if err != nil {
				c.logger.Error("poison pill detected, skipping message",
					slog.Any("error", err),
					slog.Int64("offset", msg.Offset),
				)
				if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
					c.logger.Error("failed to commit poison pill", slog.Any("error", commitErr))
				}
				continue
			}
			// Добавляем в батч
			events = append(events, event)
			kafkaMessages = append(kafkaMessages, msg)

			// Если батч заполнился — отправляем в ClickHouse
			if len(events) >= batchSize {
				if err := c.flushBatch(ctx, events, kafkaMessages); err != nil {
					c.logger.Error("failed to flush full batch", slog.Any("error", err))
				}
				events = events[:0]
				kafkaMessages = kafkaMessages[:0]
				ticker.Reset(flushTimeout) // Сбрасываем таймер
			}
		}
	}
}

// Валидация и парсинг события
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

// Отправка пачки в ClickHouse и коммитит её в Kafka
func (c *Consumer) flushBatch(ctx context.Context, events []models.TransactionEvent, messages []kafkaGo.Message) error {
	receivedAt := time.Now().UTC()

	// Пишем в ClickHouse пачку сообщений
	if err := c.service.ProcessBatch(ctx, events, receivedAt); err != nil {
		return err
	}

	// Коммитим пачку в Kafka
	if err := c.reader.CommitMessages(ctx, messages...); err != nil {
		return err
	}

	c.logger.Info("successfully processed batch", slog.Int("count", len(events)))
	return nil
}
