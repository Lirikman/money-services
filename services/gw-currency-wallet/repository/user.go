package repository

import (
	"context"

	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
)

type UserRepository interface {
	Create(ctx context.Context, u *models.User) error
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	GetUserByEmail(ctx context.Context, username string) (*models.User, error)
}
