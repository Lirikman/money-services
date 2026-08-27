package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Lirikman/money_services/services/gw-analytics/models"
)

type ClickHouseRepository struct {
	conn clickhouse.Conn
}

// Создание нового репозитория clickHouse
// Подключение к БД
func NewClickHouse(addr, database string) (*ClickHouseRepository, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},

		Auth: clickhouse.Auth{
			Database: database,
			Username: "default",
			Password: "",
		},
	})

	if err != nil {
		return nil, err
	}

	return &ClickHouseRepository{
		conn: conn,
	}, nil
}

// Сохранение транзакции
func (r *ClickHouseRepository) SaveTransaction(ctx context.Context, event models.TransactionEvent, receivedAt time.Time) error {
	latency := receivedAt.Sub(event.CreatedAt)

	if latency < 0 {
		latency = 0
	}

	return r.conn.Exec(ctx, `
		INSERT INTO analytics.transaction_events
		(
			transaction_id,
			user_id,
			operation,
			status,
			amount,
			currency,
			created_at,
			received_at,
			latency_ms,
			retry_count,
			error,
			version
		)
		VALUES
		(
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?
		)
	`,
		event.TransactionID,
		event.UserID,
		event.Operation,
		event.Status,
		event.Amount,
		event.Currency,
		event.CreatedAt,
		receivedAt,
		uint64(latency.Milliseconds()),
		event.RetryCount,
		event.Error,
		uint64(receivedAt.UnixNano()),
	)
}
