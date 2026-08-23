package service

import (
	"context"
	"errors"
	"strconv"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/kafka"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/repository"
	"github.com/google/uuid"
)

type WalletService struct {
	repo       repository.WalletRepository
	grpcClient repository.CurrencyClient
	producer   kafka.Producer
}

var (
	ErrInvalidAmount    = errors.New("Invalid transaction amount")
	ErrInvalidCurrency  = errors.New("Invalid or unsupported currency")
	ErrSameCurrency     = errors.New("Source and target currencies must be different")
	ErrGenTransactionID = errors.New("Error generating transaction ID")
	ErrKafkaSend        = errors.New("kafka send error")
)

// Создание нового сервиса кошелька
func NewWalletService(repo repository.WalletRepository, grpcClient repository.CurrencyClient, producer kafka.Producer) *WalletService {
	return &WalletService{repo: repo, grpcClient: grpcClient, producer: producer}
}

// Пополнение кошелька
func (s *WalletService) Deposit(ctx context.Context, userID int64, currency string, amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if !isSupportedCurrency(currency) {
		return ErrInvalidCurrency
	}
	// зачисляем деньги в БД
	if err := s.repo.Deposit(ctx, userID, currency, amount); err != nil {
		return err
	}
	// отправляем сообщение в kafka
	transID, err := genTransID()
	if err != nil {
		return err
	}

	transDeposit := models.Transaction{
		TransactionID: transID.String(),
		UserID:        strconv.FormatInt(userID, 10),
		Operation:     "deposit",
		Amount:        amount,
		Currency:      currency,
	}
	if prodErr := s.producer.Send(ctx, transDeposit); prodErr != nil {
		return ErrKafkaSend
	}

	return nil
}

// Вывод средств с кошелька
func (s *WalletService) Withdraw(ctx context.Context, userID int64, currency string, amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if !isSupportedCurrency(currency) {
		return ErrInvalidCurrency
	}
	// списываем деньги в БД
	if err := s.repo.Withdraw(ctx, userID, currency, amount); err != nil {
		return err
	}
	// отправляем сообщение в kafka
	transID, err := genTransID()
	if err != nil {
		return err
	}

	transWithdraw := models.Transaction{
		TransactionID: transID.String(),
		UserID:        strconv.FormatInt(userID, 10),
		Operation:     "withdraw",
		Amount:        amount,
		Currency:      currency,
	}
	if prodErr := s.producer.Send(ctx, transWithdraw); prodErr != nil {
		return ErrKafkaSend
	}
	return nil
}

// Получение курсов обмена валют
func (s *WalletService) GetRates(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error) {
	return s.grpcClient.GetRates(ctx, req)
}

// Обмен валюты
func (s *WalletService) Exchange(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if !isSupportedCurrency(fromCur) && !isSupportedCurrency(toCur) {
		return ErrInvalidCurrency
	}
	if fromCur == toCur {
		return ErrSameCurrency
	}

	rate, err := s.grpcClient.GetRateCurrency(ctx, fromCur, toCur)

	if err != nil {
		return err
	}

	targetAmount := amount * rate

	// обмениваем валюту в БД
	if err := s.repo.Exchange(ctx, userID, fromCur, toCur, amount, targetAmount); err != nil {
		return err
	}
	// отпрвляем сообщение в kafka
	transID, err := genTransID()
	if err != nil {
		return err
	}

	transExchange := models.Transaction{
		TransactionID: transID.String(),
		UserID:        strconv.FormatInt(userID, 10),
		Operation:     "exchange",
		Amount:        amount,
		Currency:      fromCur,
		FromCurrency:  fromCur,
		ToCurrency:    toCur,
		Rate:          rate,
	}
	if prodErr := s.producer.Send(ctx, transExchange); prodErr != nil {
		return ErrKafkaSend
	}
	return nil
}

// Получение баланса пользователя
func (s *WalletService) GetBalances(ctx context.Context, userID int64) (map[string]string, error) {
	return s.repo.GetBalances(ctx, userID)
}

func isSupportedCurrency(cur string) bool {
	return cur == "USD" || cur == "RUB" || cur == "EUR"
}

func genTransID() (uuid.UUID, error) {
	transID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, ErrGenTransactionID
	}
	return transID, nil
}
