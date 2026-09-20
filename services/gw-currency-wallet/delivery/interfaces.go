package http

import (
	"context"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
)

type UserService interface {
	Register(ctx context.Context, username, email, password string) error
	Login(ctx context.Context, username, password string) (string, error)
}

type WalletService interface {
	Deposit(ctx context.Context, userID int64, currency string, amount float64) error
	Withdraw(ctx context.Context, userID int64, currency string, amount float64) error
	GetRates(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error)
	Exchange(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error
	GetBalances(ctx context.Context, userID int64) (map[string]string, error)
	RetrySendAnalytics(ctx context.Context, event models.TransactionEvent, initialError string)
}

type BalanceService interface {
	GetBalances(ctx context.Context, userID int64) (map[string]string, error)
}
