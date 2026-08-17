package repository

import (
	"context"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
)

type CurrencyClient interface {
	GetRates(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error)
	GetRateCurrency(ctx context.Context, from, to string) (float64, error)
}
