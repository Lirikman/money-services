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

// Запуск сохранения транзакций
func (s *AnalyticsService) Process(ctx context.Context, event models.TransactionEvent, receivedAt time.Time) error {
	return s.repository.SaveTransaction(ctx, event, receivedAt)
}
