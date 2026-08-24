package repository

import (
	"context"

	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
)

type Producer interface {
	Send(ctx context.Context, transfer models.Transaction) error
	Close() error
}
