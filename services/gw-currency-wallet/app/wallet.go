package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/repository"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type WalletService struct {
	repo          repository.WalletRepository
	grpcClient    repository.CurrencyClient
	kafkaProducer repository.Producer
}

var (
	ErrInvalidAmount    = errors.New("Invalid transaction amount")
	ErrInvalidCurrency  = errors.New("Invalid or unsupported currency")
	ErrSameCurrency     = errors.New("Source and target currencies must be different")
	ErrGenTransactionID = errors.New("Error generating transaction ID")
	ErrKafkaSend        = errors.New("kafka send error")
)

// Создание нового сервиса кошелька
func NewWalletService(repo repository.WalletRepository, grpcClient repository.CurrencyClient, producer repository.Producer) *WalletService {
	return &WalletService{repo: repo, grpcClient: grpcClient, kafkaProducer: producer}
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
	// отправляем сообщения в kafka
	transID, err := genTransID()
	if err != nil {
		return err
	}
	transIDStr := transID.String()
	userIDStr := strconv.FormatInt(userID, 10)

	// сообщение для сервиса Notification
	notificationEvent := models.Transaction{
		TransactionID: transIDStr,
		UserID:        userIDStr,
		Operation:     "deposit",
		Amount:        amount,
		Currency:      currency,
		CreatedAt:     time.Now().UTC(),
	}
	if prodErr := s.kafkaProducer.SendNotification(ctx, notificationEvent); prodErr != nil {
		log.Printf("transaction deposit - kafka send notification error: %v", prodErr)
	} else {
		log.Printf("transaction deposit - kafka send notification successful for ID: %s", transIDStr)
	}

	// сообщение для сервиса Analytics
	analyticsEvent := models.TransactionEvent{
		TransactionID: transIDStr,
		UserID:        userIDStr,
		Operation:     "deposit",
		CreatedAt:     time.Now().UTC(),
		Status:        "success",
		RetryCount:    0,
		Error:         "",
	}
	if prodErr := s.kafkaProducer.SendAnalytics(ctx, analyticsEvent); prodErr != nil {
		log.Printf("transaction deposit - kafka send analytics error: %v", prodErr)

		// Запускаем ретраи асинхронно в goroutine
		go s.retrySendAnalytics(context.Background(), analyticsEvent, prodErr.Error())
	} else {
		log.Printf("transaction deposit - kafka send analytics successful for ID: %s", transIDStr)
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
	transIDStr := transID.String()
	userIDStr := strconv.FormatInt(userID, 10)

	// сообщение для сервиса Notification
	notificationEvent := models.Transaction{
		TransactionID: transIDStr,
		UserID:        userIDStr,
		Operation:     "withdraw",
		Amount:        amount,
		Currency:      currency,
		CreatedAt:     time.Now().UTC(),
	}
	if prodErr := s.kafkaProducer.SendNotification(ctx, notificationEvent); prodErr != nil {
		log.Printf("transaction withdraw - kafka send notification error: %v", prodErr)
	} else {
		log.Printf("transaction withdraw - kafka send notification successful for ID: %s", transIDStr)
	}

	// сообщение для сервиса Analytics
	analyticsEvent := models.TransactionEvent{
		TransactionID: transIDStr,
		UserID:        userIDStr,
		Operation:     "withdraw",
		CreatedAt:     time.Now().UTC(),
		Status:        "success",
		RetryCount:    0,
		Error:         "",
	}
	if prodErr := s.kafkaProducer.SendAnalytics(ctx, analyticsEvent); prodErr != nil {
		log.Printf("transaction withdraw - kafka send analytics error: %v", prodErr)

		// Запускаем ретраи асинхронно в goroutine
		go s.retrySendAnalytics(context.Background(), analyticsEvent, prodErr.Error())
	} else {
		log.Printf("transaction withdraw - kafka send analytics successful for ID: %s", transIDStr)
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

	// округляем до 2 знаков в строку и конвертируем в Decimal128
	rateStr := fmt.Sprintf("%.2f", rate)
	mongoRate, err := primitive.ParseDecimal128(rateStr)
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
	transIDStr := transID.String()
	userIDStr := strconv.FormatInt(userID, 10)

	// сообщение для сервиса Notification
	notificationEvent := models.Transaction{
		TransactionID: transIDStr,
		UserID:        userIDStr,
		Operation:     "exchange",
		Amount:        amount,
		Currency:      fromCur,
		FromCurrency:  fromCur,
		ToCurrency:    toCur,
		Rate:          mongoRate,
		CreatedAt:     time.Now().UTC(),
	}
	if prodErr := s.kafkaProducer.SendNotification(ctx, notificationEvent); prodErr != nil {
		log.Printf("transacion exchange - kafka send notification error: %v", prodErr)
	} else {
		log.Printf("transaction exchange - kafka send notification successful for ID: %s", transIDStr)
	}

	// сообщение для сервиса Analytics
	analyticsEvent := models.TransactionEvent{
		TransactionID: transIDStr,
		UserID:        userIDStr,
		Operation:     "exchange",
		CreatedAt:     time.Now().UTC(),
		Status:        "success",
		RetryCount:    0,
		Error:         "",
	}
	if prodErr := s.kafkaProducer.SendAnalytics(ctx, analyticsEvent); prodErr != nil {
		log.Printf("transaction exchange - kafka send analytics error: %v", prodErr)

		// Запускаем ретраи асинхронно в goroutine
		go s.retrySendAnalytics(context.Background(), analyticsEvent, prodErr.Error())
	} else {
		log.Printf("transaction exchange - kafka send analytics successful for ID: %s", transIDStr)
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

// обработка ретраев
func (s *WalletService) retrySendAnalytics(ctx context.Context, event models.TransactionEvent, initialError string) {
	const maxRetries = 5

	// задаём начальную паузу между попытками
	backoff := 1 * time.Second

	event.Status = "error"
	event.Error = initialError

	for event.RetryCount < maxRetries {
		event.RetryCount++
		event.CreatedAt = time.Now().UTC()

		select {
		case <-ctx.Done():
			log.Printf("retry context canceled for tx %s", event.TransactionID)
			return
		case <-time.After(backoff):
		}

		log.Printf("retrying analytics send for tx %s (attempt %d/%d)...", event.TransactionID, event.RetryCount, maxRetries)

		if err := s.kafkaProducer.SendAnalytics(ctx, event); err == nil {
			log.Printf("analytics event for tx %s successfully sent after %d retries", event.TransactionID, event.RetryCount)
			return
		} else {
			event.Error = err.Error()
			backoff *= 2 // Увеличиваем паузу в 2 раза
		}
	}

	// / если все попытки исчерпаны
	log.Printf("CRITICAL: failed to send analytics for tx %s after %d attempts. Last error: %s",
		event.TransactionID, maxRetries, event.Error)
}
