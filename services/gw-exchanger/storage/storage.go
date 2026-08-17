package storage

import "context"

type Repository interface {
	GetRateCurrency(
		ctx context.Context,
		from_currency string,
		to_currency string,
	) (float32, error)
	GetRates(ctx context.Context) (map[string]float32, error)
}
