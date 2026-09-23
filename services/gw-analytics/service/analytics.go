package service

import (
	"context"
	"time"

	repository "github.com/Lirikman/money_services/services/gw-analytics/repository/clickhouse"
	m "github.com/Lirikman/money_services/services/gw-currency-wallet/models"
)

type AnalyticsService struct {
	repository *repository.ClickHouseRepository
}

func NewAnalyticsService(repository *repository.ClickHouseRepository) *AnalyticsService {
	return &AnalyticsService{
		repository: repository,
	}
}

func (s *AnalyticsService) ProcessBatch(ctx context.Context, events []m.TransactionEvent, receivedAt time.Time) error {
	if len(events) == 0 {
		return nil
	}
	return s.repository.ProcessBatch(ctx, events, receivedAt)
}
