package repository

import (
	"context"
)

type WalletRepository interface {
	Deposit(ctx context.Context, userID int64, currency string, amount float64) error
	Withdraw(ctx context.Context, userID int64, currency string, amount float64) error
	Exchange(ctx context.Context, userID int64, fromCur, toCur string, amount, targetAmount float64) error
	GetBalances(ctx context.Context, userID int64) (map[string]string, error)
}
