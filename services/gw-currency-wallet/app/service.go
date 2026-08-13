package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/repository"
	client "github.com/Lirikman/money_services/services/gw-currency-wallet/transport"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

type WalletService struct {
	repo       *repository.Wallet
	grpcClient *client.CurrencyClient
	jwtSecret  []byte
}

// Создание нового сервиса кошелька
func NewWalletService(repo *repository.Wallet, grpcClient *client.CurrencyClient, secret string) *WalletService {
	return &WalletService{repo: repo, grpcClient: grpcClient, jwtSecret: []byte(secret)}
}

// Регистрация нового пользователя
func (s *WalletService) Register(ctx context.Context, username, email, password string) (int64, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	id, err := s.repo.CreateUser(ctx, username, email, string(hashed))
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Авторизация пользователя
func (s *WalletService) Login(ctx context.Context, username, password string) (string, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil || user == nil {
		return "", errors.New("unable to obtain user credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	return token.SignedString(s.jwtSecret)
}

// Пополнение кошелька
func (s *WalletService) Deposit(ctx context.Context, userID int64, currency string, amount float64) error {
	return s.repo.Deposit(ctx, userID, currency, amount)
}

// Вывод средств с кошелька
func (s *WalletService) Withdraw(ctx context.Context, userID int64, currency string, amount float64) error {
	return s.repo.Withdraw(ctx, userID, currency, amount)
}

// Получение курсов обмена валют
func (s *WalletService) GetRates(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error) {
	return s.grpcClient.GetRates(ctx, req)
}

// Обмен валюты
func (s *WalletService) Exchange(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error {
	rate, err := s.grpcClient.GetRateCurrency(ctx, fromCur, toCur)

	if err != nil {
		return fmt.Errorf("failed to get currency rate via gRPC for user %d: %w", userID, err)
	}

	targetAmount := amount * rate
	return s.repo.Exchange(ctx, userID, fromCur, toCur, amount, targetAmount)
}

// Получение баланса пользователя
func (s *WalletService) GetBalances(ctx context.Context, userID int64) (map[string]string, error) {
	return s.repo.GetBalances(ctx, userID)
}
