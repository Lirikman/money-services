package storage

import "context"

type Repository interface {
	GetRate(
		ctx context.Context,
		from_currency string,
		to_currency string,
	) (float32, error)
	GetRateCurrency(ctx context.Context) (map[string]float32, error)
}
