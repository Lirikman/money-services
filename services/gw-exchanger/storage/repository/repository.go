package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Lirikman/money_services/services/gw-exchanger/storage"
)

type Repository struct {
	db  *sql.DB
	log *slog.Logger
}

func NewPostgresRepository(db *sql.DB, log *slog.Logger) storage.Repository {
	return &Repository{db: db, log: log}
}

func (r *Repository) GetRate(
	ctx context.Context,
	base_currency string,
	target_currency string,
) (float32, error) {

	r.log.Info(
		"loading exchange rate",
		slog.String("base_currency", base_currency),
		slog.String("target_currency", target_currency),
	)

	var rate float32

	err := r.db.QueryRowContext(
		ctx,
		`
        SELECT rate
        FROM exchange_rates
        WHERE base_currency=$1
          AND target_currency=$2
        `,
		base_currency,
		target_currency,
	).Scan(&rate)

	if err != nil {
		r.log.Error(
			"db query failed",
			slog.Any("error", err),
		)
		return 0, err
	}

	return rate, err
}

func (r *Repository) GetRateCurrency(ctx context.Context) (map[string]float32, error) {

	result := make(map[string]float32)

	rows, err := r.db.QueryContext(ctx, `SELECT base_currency, target_currency, rate FROM exchange_rates`)

	if err != nil {
		return nil, fmt.Errorf("query exchange rates failed: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		var baseCurrency, targeCurrency string
		var rate float32

		if err := rows.Scan(&baseCurrency, &targeCurrency, &rate); err != nil {
			return nil, fmt.Errorf("scan exchange rate failed: %w", err)
		}

		key := baseCurrency + "/" + targeCurrency
		result[key] = rate
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return result, nil
}
