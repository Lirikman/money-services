package service

import (
	"context"
	"time"

	"github.com/Lirikman/money_services/services/gw-analytics/models"
	repository "github.com/Lirikman/money_services/services/gw-analytics/repository/clickhouse"
)

type AnalyticsService struct {
	repository *repository.ClickHouseRepository
}

// Создание сервиса аналитики
func NewAnalyticsService(repository *repository.ClickHouseRepository) *AnalyticsService {
	return &AnalyticsService{
		repository: repository,
	}
}

// Запуск сохранения событий
func (s *AnalyticsService) ProcessBatch(ctx context.Context, events []models.TransactionEvent, receivedAt time.Time) error {
	if len(events) == 0 {
		return nil
	}
	return s.repository.SaveTransactionBatch(ctx, events, receivedAt)
}
