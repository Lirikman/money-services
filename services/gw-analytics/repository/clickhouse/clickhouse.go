package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/Lirikman/money_services/services/gw-analytics/models"
)

type ClickHouseRepository struct {
	conn clickhouse.Conn
}

// Создание нового репозитория clickHouse
// Подключение к БД
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

// Пакетное сохранение событий
func (r *ClickHouseRepository) SaveTransactionBatch(ctx context.Context, events []models.TransactionEvent, receivedAt time.Time) error {
	// Инициализируем пакетную вставку
	batch, err := r.conn.PrepareBatch(ctx, `
		INSERT INTO analytics.transaction_events 
		(
			transaction_id, 
			user_id, 
			operation, 
			status, 
			created_at, 
			received_at, 
			latency_ms, 
			retry_count, 
			error, 
			version
		)
	`)

	if err != nil {
		return fmt.Errorf("failed to prepare clickhouse batch: %w", err)
	}

	defer func() {
		_ = batch.Abort()
	}()

	// Наполняем пакет данными в цикле
	for _, event := range events {
		// Расчет задержки доставки
		latencySub := max(receivedAt.Sub(event.CreatedAt), 0)
		latencyMs := int64(latencySub.Milliseconds())

		// Добавляем строку в буфер пакета.
		err = batch.Append(
			event.TransactionID,
			event.UserID,
			event.Operation,
			event.Status,
			event.CreatedAt,
			receivedAt,
			latencyMs,
			event.RetryCount,
			event.Error,
			uint64(receivedAt.UnixNano()),
		)
		if err != nil {
			return fmt.Errorf("failed to append row to clickhouse batch: %w", err)
		}
	}

	// Отправляем всю пачку в ClickHouse
	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch to clickhouse: %w", err)
	}

	return nil
}

// Проверка запуска ClickHouse
func (r *ClickHouseRepository) Ping(ctx context.Context) error {
	return r.conn.Ping(ctx)
}
