package repository

import (
	"context"

	"github.com/Lirikman/money_services/services/gw-notification/models"
)

type TransactionRepository interface {
	Save(ctx context.Context, transaction models.Transaction) error
	SaveBatch(ctx context.Context, transactions []models.Transaction) error
	Close(ctx context.Context) error
}
