package repository

import (
	"context"

	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
)

type Producer interface {
	SendNotification(ctx context.Context, transfer models.Transaction) error
	SendAnalytics(ctx context.Context, analytics models.TransactionEvent) error
	Close() error
}
