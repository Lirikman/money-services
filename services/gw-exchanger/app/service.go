package app

import (
	"context"
	"log/slog"

	"github.com/Lirikman/money_services/services/gw-exchanger/storage"
)

type Service struct {
	repo storage.Repository
	log  *slog.Logger
}

func New(repo storage.Repository, log *slog.Logger) *Service {
	return &Service{
		repo: repo,
		log:  log,
	}
}

func (s *Service) GetRate(
	ctx context.Context,
	from_currency string,
	to_currency string,
) (float32, error) {

	return s.repo.GetRate(ctx, from_currency, to_currency)
}

func (s *Service) GetRateCurrency(
	ctx context.Context,
) (map[string]float32, error) {

	return s.repo.GetRateCurrency(ctx)
}
