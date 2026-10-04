package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
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

type kafkaResult struct {
	msg kafkago.Message
	err error
}

func (s *NotificationService) Run(ctx context.Context) error {

	s.logger.Info("notification service started",
		slog.Int("batch_size", s.batchSize),
		slog.Duration("batch_timeout", s.batchTimeout),
	)

	batch := make([]batchMessage, 0, s.batchSize)
	timer := time.NewTimer(s.batchTimeout)
	defer timer.Stop()

	s.logger.Info("KAFKA CONSUMER STARTED")

	msgChan := make(chan kafkaResult, s.batchSize)

	var wg sync.WaitGroup

	consumerCtx, cancelConsumer := context.WithCancel(ctx)
	defer cancelConsumer()

	wg.Add(1)

	go func() {
		defer wg.Done()
		for {
			msg, err := s.consumer.Fetch(consumerCtx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				select {
				case <-consumerCtx.Done():
					return
				case msgChan <- kafkaResult{msg: msg, err: err}:
				}
				continue
			}
			select {
			case <-consumerCtx.Done():
				return

			case msgChan <- kafkaResult{msg: msg, err: nil}:
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("stopping notification consumer")
			cancelConsumer()
			wg.Wait()
			if len(batch) > 0 {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				err := s.flush(shutdownCtx, batch)
				cancel()
				if err != nil {
					return fmt.Errorf("flush batch during shutdown: %w", err)
				}
				batch = batch[:0]
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

		case res := <-msgChan:
			if res.err != nil {
				if errors.Is(res.err, context.Canceled) {
					continue
				}
				s.logger.Error("failed to fetch kafka message", slog.Any("error", res.err))
				continue
			}

			message := res.msg

			var transaction models.Transaction
			if err := json.Unmarshal(message.Value, &transaction); err != nil {
				s.logger.Error("invalid kafka message",
					slog.Int("partition", message.Partition),
					slog.Int64("offset", message.Offset),
					slog.Any("error", err),
				)
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
				if commitErr := s.consumer.Commit(ctx, message); commitErr != nil {
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
}

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
		return fmt.Errorf("save transaction batch: %w", err)
	}

	if err := s.consumer.Commit(ctx, messages...); err != nil {
		logger.Error("failed to commit kafka offsets", slog.Any("error", err))
		return fmt.Errorf("commit offsets: %w", err)
	}

	logger.Info("transaction batch successfully saved and committed")

	return nil
}
