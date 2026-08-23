package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Lirikman/money_services/services/gw-notification/kafka"
	"github.com/Lirikman/money_services/services/gw-notification/models"
	"github.com/Lirikman/money_services/services/gw-notification/repository"

	kafkago "github.com/segmentio/kafka-go"
)

type NotificationService struct {
	consumer *kafka.Consumer
	repo     repository.TransactionRepository
	logger   *slog.Logger

	batchSize    int
	batchTimeout time.Duration
}

type batchMessage struct {
	message     kafkago.Message
	transaction models.Transaction
}

// Создание нового сервиса сохранения переводов
func NewNotificationService(
	consumer *kafka.Consumer,
	repo repository.TransactionRepository,
	logger *slog.Logger,
	batchSize int,
	batchTimeout time.Duration,
) *NotificationService {

	return &NotificationService{
		consumer: consumer,
		repo:     repo,
		logger:   logger,

		batchSize:    batchSize,
		batchTimeout: batchTimeout,
	}
}

// Запуск чтения сообщений из kafka
func (s *NotificationService) Run(ctx context.Context) error {

	s.logger.Info("notification service started",
		slog.Int("batch_size", s.batchSize),
		slog.Duration("batch_timeout", s.batchTimeout),
	)

	batch := make([]batchMessage, 0, s.batchSize)

	timer := time.NewTimer(s.batchTimeout)

	defer timer.Stop()

	for {
		select {

		case <-ctx.Done():

			if len(batch) > 0 {
				if err := s.flush(ctx, batch); err != nil {
					return err
				}
			}

			s.logger.Info("notification service stopped")

			return nil

		case <-timer.C:

			if len(batch) > 0 {
				if err := s.flush(ctx, batch); err != nil {
					return err
				}

				batch = batch[:0]
			}

			timer.Reset(s.batchTimeout)

		default:
		}

		message, err := s.consumer.Fetch(ctx)

		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			s.logger.Error("failed to fetch kafka message", slog.Any("error", err))

			continue
		}

		var transaction models.Transaction
		if err := json.Unmarshal(message.Value, &transaction); err != nil {
			s.logger.Error("invalid kafka message",
				slog.Int("partition", message.Partition),
				slog.Int64("offset", message.Offset),
				slog.Any("error", err),
			)
			// Некорректное сообщение невозможно обработать.
			// Commit нужен, чтобы оно не блокировало partition.
			if err := s.consumer.Commit(ctx, message); err != nil {
				return err
			}

			continue
		}

		logger := s.logger.With(
			slog.String("transaction_id", transaction.TransactionID),
			slog.String("user_id", transaction.UserID),
			slog.String("operation", string(transaction.Operation)),
			slog.Float64("amount", transaction.Amount),
		)

		if err := transaction.Validate(); err != nil {

			logger.Debug("invalid transaction",
				slog.String("transaction_id", transaction.TransactionID),
				slog.Any("error", err),
			)

			if commitErr := s.consumer.Commit(ctx, message); err != nil {
				return commitErr
			}

			continue
		}

		logger.Info("large transaction received")

		batch = append(batch, batchMessage{
			message:     message,
			transaction: transaction,
		},
		)

		if len(batch) >= s.batchSize {

			if err := s.flush(ctx, batch); err != nil {
				return err
			}

			batch = batch[:0]

			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}

			timer.Reset(s.batchTimeout)
		}
	}
}

// Сохранение денежных переводов в БД
func (s *NotificationService) flush(ctx context.Context, batch []batchMessage) error {

	if len(batch) == 0 {
		return nil
	}

	logger := s.logger.With(slog.Int("batch_size", len(batch)))

	logger.Info("saving transaction batch")

	transactions := make([]models.Transaction, 0, len(batch))

	messages := make([]kafkago.Message, 0, len(batch))

	for _, item := range batch {
		transactions = append(transactions, item.transaction)
		messages = append(messages, item.message)
	}

	if err := s.repo.SaveBatch(ctx, transactions); err != nil {

		logger.Error("failed to save transaction batch", slog.Any("error", err))

		// offset НЕ commit
		// Kafka доставит сообщения снова
		return fmt.Errorf("save transaction batch: %w", err)
	}

	if err := s.consumer.Commit(ctx, messages...); err != nil {

		logger.Error("failed to commit kafka offsets", slog.Any("error", err))

		return fmt.Errorf("commit offsets: %w", err)
	}

	logger.Info("transaction batch successfully saved and committed")

	return nil
}
