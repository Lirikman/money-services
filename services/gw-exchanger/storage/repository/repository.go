package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Lirikman/money_services/services/gw-exchanger/storage"
)

type Repository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) storage.Repository {
	return &Repository{db: db}
}

func (r *Repository) GetRateCurrency(
	ctx context.Context,
	base_currency string,
	target_currency string) (float32, error) {

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
		return 0, fmt.Errorf("query exchange rate %s/%s failed: %w", base_currency, target_currency, err)
	}

	return rate, nil
}

func (r *Repository) GetRates(ctx context.Context) (map[string]float32, error) {

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
