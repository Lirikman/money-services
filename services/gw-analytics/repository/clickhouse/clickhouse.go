package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Lirikman/money_services/services/gw-analytics/models"
	m "github.com/Lirikman/money_services/services/gw-currency-wallet/models"
)

type ClickHouseRepository struct {
	conn clickhouse.Conn
}

func NewClickHouse(addr, database, username, password string) (*ClickHouseRepository, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},

		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},

		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},

		DialTimeout: 5 * time.Second,
	})

	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}

	return &ClickHouseRepository{conn: conn}, nil
}

func (s *ClickHouseRepository) ProcessBatch(ctx context.Context, events []m.TransactionEvent, receivedAt time.Time) error {
	if len(events) == 0 {
		return nil
	}

	analyticsEvents := make([]models.AnalyticsEvent, 0, len(events))

	for _, event := range events {
		latency := max(receivedAt.Sub(event.CreatedAt), 0)

		analyticsEvents = append(
			analyticsEvents,
			models.AnalyticsEvent{
				TransactionID: event.TransactionID,
				UserID:        event.UserID,
				Operation:     event.Operation,
				Status:        event.Status,
				CreatedAt:     event.CreatedAt,
				ReceivedAt:    receivedAt,
				LatencyMs:     uint64(latency.Milliseconds()),
				RetryCount:    uint32(event.RetryCount),
				Error:         event.Error,
			},
		)
	}
	return s.InsertEvents(ctx, analyticsEvents)
}

func (r *ClickHouseRepository) InsertEvents(ctx context.Context, events []models.AnalyticsEvent) error {
	if len(events) == 0 {
		return nil
	}

	batch, err := r.conn.PrepareBatch(
		ctx,
		`INSERT INTO transaction_events
		(
			transaction_id,
			user_id,
			operation,
			status,
			created_at,
			received_at,
			latency_ms,
			retry_count,
			error
		)`,
	)
	if err != nil {
		return fmt.Errorf("prepare clickhouse batch: %w", err)
	}

	for _, event := range events {
		if err := batch.Append(
			event.TransactionID,
			event.UserID,
			event.Operation,
			event.Status,
			event.CreatedAt,
			event.ReceivedAt,
			event.LatencyMs,
			event.RetryCount,
			event.Error,
		); err != nil {
			return fmt.Errorf("append clickhouse event: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send clickhouse batch: %w", err)
	}

	return nil
}

func (r *ClickHouseRepository) Ping(ctx context.Context) error {
	return r.conn.Ping(ctx)
}
