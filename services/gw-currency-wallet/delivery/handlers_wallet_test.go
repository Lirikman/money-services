package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	service "github.com/Lirikman/money_services/services/gw-currency-wallet/app"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
	wallRep "github.com/Lirikman/money_services/services/gw-currency-wallet/repository/postgres"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Fake WalletService для тестов
type mockWalletService struct {
	depositFunc            func(ctx context.Context, userID int64, currency string, amount float64) error
	withdrawFunc           func(ctx context.Context, userID int64, currency string, amount float64) error
	getRatesFunc           func(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error)
	exchangeFunc           func(ctx context.Context, userID int64, fromCur string, toCur string, amount float64) error
	getBalancesFunc        func(ctx context.Context, userID int64) (map[string]string, error)
	retrySendAnalyticsFunc func(ctx context.Context, event models.TransactionEvent, initialError string)

	// Для проверки вызовов
	calls []string

	userID   int64
	currency string
	amount   float64
	fromCur  string
	toCur    string
}

func (m *mockWalletService) Deposit(ctx context.Context, userID int64, currency string, amount float64) error {
	m.calls = append(m.calls, "Deposit")
	m.userID = userID
	m.currency = currency
	m.amount = amount
	if m.depositFunc != nil {
		return m.depositFunc(ctx, userID, currency, amount)
	}
	return nil
}

func (m *mockWalletService) Withdraw(ctx context.Context, userID int64, currency string, amount float64) error {
	m.calls = append(m.calls, "Withdraw")
	m.userID = userID
	m.currency = currency
	m.amount = amount
	if m.withdrawFunc != nil {
		return m.withdrawFunc(ctx, userID, currency, amount)
	}
	return nil
}

func (m *mockWalletService) GetRates(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error) {
	m.calls = append(m.calls, "GetRates")
	if m.getRatesFunc != nil {
		return m.getRatesFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockWalletService) Exchange(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error {
	m.calls = append(m.calls, "Exchange")
	m.userID = userID
	m.fromCur = fromCur
	m.toCur = toCur
	m.amount = amount
	if m.exchangeFunc != nil {
		return m.exchangeFunc(ctx, userID, fromCur, toCur, amount)
	}
	return nil
}

func (m *mockWalletService) GetBalances(ctx context.Context, userID int64) (map[string]string, error) {
	m.calls = append(m.calls, "GetBalances")
	m.userID = userID
	if m.getBalancesFunc != nil {
		return m.getBalancesFunc(ctx, userID)
	}
	return nil, nil
}

func (m *mockWalletService) RetrySendAnalytics(ctx context.Context, event models.TransactionEvent, initialError string) {
	m.calls = append(m.calls, "RetrySendAnalytics")
	if m.retrySendAnalyticsFunc != nil {
		m.retrySendAnalyticsFunc(ctx, event, initialError)
	}
}

func newHandler(mockWall *mockWalletService) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHandler(mockWall, nil, logger)
}

func (m *mockWalletService) assertCalls(t *testing.T, expected ...string) {
	t.Helper()

	if !reflect.DeepEqual(m.calls, expected) {
		t.Fatalf("expected calls %v, got %v", expected, m.calls)
	}
}

func TestHandler_Balance(t *testing.T) {

	t.Run("success", func(t *testing.T) {
		expectedBalance := map[string]string{
			"USD": "1500.50",
			"EUR": "750.25",
			"RUB": "100000.00",
		}

		mockSvc := &mockWalletService{
			getBalancesFunc: func(ctx context.Context, userID int64) (map[string]string, error) {
				return expectedBalance, nil
			},
		}

		handler := newHandler(mockSvc)

		req := httptest.NewRequest(http.MethodGet, "/balance", nil)

		const userID int64 = 42

		req = req.WithContext(context.WithValue(req.Context(), UserIDKey, userID))

		rec := httptest.NewRecorder()
		handler.Balance(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		mockSvc.assertCalls(t, "GetBalances")

		if mockSvc.userID != userID {
			t.Fatalf("expected userID %d, got %d", userID, mockSvc.userID)
		}

		var got BalanceResponse

		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !reflect.DeepEqual(got.Balance, expectedBalance) {
			t.Errorf("expected balance %v, got %v", expectedBalance, got.Balance)
		}
	})
}

func TestHandler_Balance_UserIDMissing(t *testing.T) {
	mockSvc := &mockWalletService{}
	handler := newHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/balance", nil)
	rec := httptest.NewRecorder()

	handler.Balance(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	mockSvc.assertCalls(t)
}

func TestHandler_Balance_InvalidUserIDType(t *testing.T) {
	mockSvc := &mockWalletService{}

	handler := newHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/balance", nil)

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, int(42)), // не int64
	)

	rec := httptest.NewRecorder()

	handler.Balance(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	mockSvc.assertCalls(t)
}

func TestHandker_Balance_ServiceError(t *testing.T) {
	expectedErr := errors.New("database connection failed")

	mockSvc := &mockWalletService{
		getBalancesFunc: func(ctx context.Context, userID int64) (map[string]string, error) {
			return nil, expectedErr
		},
	}

	handler := newHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/balance", nil)

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)
	rec := httptest.NewRecorder()

	handler.Balance(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	mockSvc.assertCalls(t, "GetBalances")
}

func TestHandler_Deposit_Success(t *testing.T) {
	expectedBalance := map[string]string{
		"USD": "1500.50",
		"EUR": "750.25",
	}

	mockSvc := &mockWalletService{
		depositFunc: func(ctx context.Context, userID int64, currency string, amount float64) error {
			return nil
		},
		getBalancesFunc: func(ctx context.Context, userID int64) (map[string]string, error) {
			return expectedBalance, nil
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"currency": "USD",
		"amount": 500.50
	}`

	req := httptest.NewRequest(http.MethodPost, "/deposit", bytes.NewBufferString(body))

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)

	rec := httptest.NewRecorder()

	handler.Deposit(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	mockSvc.assertCalls(t, "Deposit", "GetBalances")

	if mockSvc.userID != userID {
		t.Fatalf("expected userID %d, got %d", userID, mockSvc.userID)
	}

	if mockSvc.currency != "USD" {
		t.Fatalf("expected currency %q, got %q", "USD", mockSvc.currency)
	}

	if mockSvc.amount != 500.50 {
		t.Fatalf("expected amount %.2f, got %.2f", 500.50, mockSvc.amount)
	}

	var got DeptResponse

	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if got.Message != "Account topped up successfully" {
		t.Errorf("expected message %q, got %q", "Account topped up successfully", got.Message)
	}

	if !reflect.DeepEqual(got.NewBalance, expectedBalance) {
		t.Errorf("expected balance %v, got %v", expectedBalance, got.NewBalance)
	}
}

func TestHandler_Deposit_UserIDMissing(t *testing.T) {
	mockSvc := &mockWalletService{}

	handler := newHandler(mockSvc)

	body := `{
		"currency": "USD",
		"amount": 500.50
	}`

	req := httptest.NewRequest(http.MethodPost, "/deposit", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Deposit(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	mockSvc.assertCalls(t)
}

func TestHandler_Deposit_InvalidJSON(t *testing.T) {
	mockSvc := &mockWalletService{}

	handler := newHandler(mockSvc)

	body := `{
		"currency": "USD",
		"amount":
	}`

	req := httptest.NewRequest(http.MethodPost, "/deposit", bytes.NewBufferString(body))

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)

	rec := httptest.NewRecorder()

	handler.Deposit(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	mockSvc.assertCalls(t)
}

func TestHandler_Deposit_ServiceError(t *testing.T) {
	expectedErr := errors.New("invalid amount or currency")

	mockSvc := &mockWalletService{
		depositFunc: func(ctx context.Context, userID int64, currency string, amount float64) error {
			return expectedErr
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"currency": "XYZ",
		"amount": -100
	}`

	req := httptest.NewRequest(http.MethodPost, "/deposit", bytes.NewBufferString(body))

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)

	rec := httptest.NewRecorder()

	handler.Deposit(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	mockSvc.assertCalls(t, "Deposit")
}

func TestHandler_Deposit_GetBalancesError(t *testing.T) {
	expectedErr := errors.New("database connection failed")

	mockSvc := &mockWalletService{
		depositFunc: func(ctx context.Context, userID int64, currency string, amount float64) error {
			return nil
		},
		getBalancesFunc: func(ctx context.Context, userID int64) (map[string]string, error) {
			return nil, expectedErr
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"currency": "USD",
		"amount": 500
	}`

	req := httptest.NewRequest(http.MethodPost, "/deposit", bytes.NewBufferString(body))

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)

	rec := httptest.NewRecorder()

	handler.Deposit(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	mockSvc.assertCalls(t, "Deposit", "GetBalances")
}

func TestHandler_Withdraw_Success(t *testing.T) {
	expectedBalance := map[string]string{
		"USD": "500.50",
		"EUR": "750.25",
	}

	mockSvc := &mockWalletService{
		withdrawFunc: func(ctx context.Context, userID int64, currency string, amount float64) error {
			return nil
		},

		getBalancesFunc: func(ctx context.Context, userID int64) (map[string]string, error) {
			return expectedBalance, nil
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"currency": "USD",
		"amount": 500.50
	}`

	req := httptest.NewRequest(http.MethodPost, "/withdraw", bytes.NewBufferString(body))

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)

	rec := httptest.NewRecorder()

	handler.Withdraw(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	mockSvc.assertCalls(t, "Withdraw", "GetBalances")

	if mockSvc.userID != userID {
		t.Fatalf("expected userID %d, got %d", userID, mockSvc.userID)
	}

	if mockSvc.currency != "USD" {
		t.Fatalf("expected currency %q, got %q", "USD", mockSvc.currency)
	}

	if mockSvc.amount != 500.50 {
		t.Fatalf("expected amount %.2f, got %.2f", 500.50, mockSvc.amount)
	}

	var got WithResponse

	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if got.Message != "Withdrawal successful" {
		t.Errorf("expected message %q, got %q", "Withdrawal successful", got.Message)
	}

	if !reflect.DeepEqual(got.NewBalance, expectedBalance) {
		t.Errorf("expected balance %v, got %v", expectedBalance, got.NewBalance)
	}
}

func TestHandler_Withdraw_UserIDMissing(t *testing.T) {
	mockSvc := &mockWalletService{}

	handler := newHandler(mockSvc)

	body := `{
		"currency": "USD",
		"amount": 500
	}`

	req := httptest.NewRequest(http.MethodPost, "/withdraw", bytes.NewBufferString(body))

	rec := httptest.NewRecorder()
	handler.Withdraw(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	mockSvc.assertCalls(t)
}

func TestHandler_Withdraw_InvalidUserIDType(t *testing.T) {
	mockSvc := &mockWalletService{}

	handler := newHandler(mockSvc)

	body := `{
		"currency": "USD",
		"amount": 500
	}`

	req := httptest.NewRequest(http.MethodPost, "/withdraw", bytes.NewBufferString(body))

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, int(42)),
	)

	rec := httptest.NewRecorder()

	handler.Withdraw(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	mockSvc.assertCalls(t)
}

func TestHandler_Withdraw_InvalidJSON(t *testing.T) {
	mockSvc := &mockWalletService{}

	handler := newHandler(mockSvc)

	body := `{
		"currency": "USD",
		"amount":
	}`

	req := httptest.NewRequest(http.MethodPost, "/withdraw", bytes.NewBufferString(body))

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)

	rec := httptest.NewRecorder()

	handler.Withdraw(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	mockSvc.assertCalls(t)
}

func TestHandler_Withdraw_ServiceError(t *testing.T) {
	expectedErr := errors.New("insufficient funds")

	mockSvc := &mockWalletService{
		withdrawFunc: func(ctx context.Context, userID int64, currency string, amount float64) error {
			return expectedErr
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"currency": "USD",
		"amount": 1000
	}`

	req := httptest.NewRequest(http.MethodPost, "/withdraw", bytes.NewBufferString(body))

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)

	rec := httptest.NewRecorder()

	handler.Withdraw(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	mockSvc.assertCalls(t, "Withdraw")
}

func TestHandler_Withdraw_GetBalancesError(t *testing.T) {
	expectedErr := errors.New("database connection failed")

	mockSvc := &mockWalletService{
		withdrawFunc: func(ctx context.Context, userID int64, currency string, amount float64) error {
			return nil
		},

		getBalancesFunc: func(ctx context.Context, userID int64) (map[string]string, error) {
			return nil, expectedErr
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"currency": "USD",
		"amount": 500
	}`

	req := httptest.NewRequest(http.MethodPost, "/withdraw", bytes.NewBufferString(body))

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)

	rec := httptest.NewRecorder()

	handler.Withdraw(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	mockSvc.assertCalls(t, "Withdraw", "GetBalances")
}

func TestHandler_ExchangeCurrency_Success(t *testing.T) {
	expectedBalance := map[string]string{
		"USD": "900.00",
		"EUR": "850.00",
	}

	mockSvc := &mockWalletService{
		exchangeFunc: func(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error {
			return nil
		},
		getBalancesFunc: func(ctx context.Context, userID int64) (map[string]string, error) {
			return expectedBalance, nil
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"from_currency": "USD",
		"to_currency": "EUR",
		"amount": 100.00
	}`

	req := httptest.NewRequest(http.MethodPost, "/exchange", bytes.NewBufferString(body))

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)

	rec := httptest.NewRecorder()

	handler.ExchangeCurrency(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	mockSvc.assertCalls(t, "Exchange", "GetBalances")

	if mockSvc.userID != userID {
		t.Fatalf("expected userID %d, got %d", userID, mockSvc.userID)
	}

	if mockSvc.fromCur != "USD" {
		t.Fatalf("expected from currency %q, got %q", "USD", mockSvc.fromCur)
	}

	if mockSvc.toCur != "EUR" {
		t.Fatalf("expected to currency %q, got %q", "EUR", mockSvc.toCur)
	}

	if mockSvc.amount != 100.00 {
		t.Fatalf("expected amount %.2f, got %.2f", 100.00, mockSvc.amount)
	}

	var got ExchangeResponse

	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if got.Message != "Exchange successful" {
		t.Errorf("expected message %q, got %q", "Exchange successful", got.Message)
	}

	if got.ExchangedAmount != 100.00 {
		t.Errorf("expected exchanged amount %.2f, got %.2f", 100.00, got.ExchangedAmount)
	}

	if !reflect.DeepEqual(got.NewBalance, expectedBalance) {
		t.Errorf("expected balance %v, got %v", expectedBalance, got.NewBalance)
	}
}

func TestHandler_ExchangeCurrency_Unauthorized(t *testing.T) {
	mockSvc := &mockWalletService{
		exchangeFunc: func(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error {
			return nil
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"from_currency": "USD",
		"to_currency": "EUR",
		"amount": 100
	}`

	req := httptest.NewRequest(http.MethodPost, "/exchange", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ExchangeCurrency(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	if len(mockSvc.calls) != 0 {
		t.Fatalf("expected no service calls, got %v", mockSvc.calls)
	}
}

func TestHandler_ExchangeCurrency_InvalidBody(t *testing.T) {
	mockSvc := &mockWalletService{}

	handler := newHandler(mockSvc)

	body := `{
		"from_currency": "USD",
		"to_currency": "EUR",
		"amount":
	}`

	req := httptest.NewRequest(http.MethodPost, "/exchange", bytes.NewBufferString(body))

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)

	rec := httptest.NewRecorder()

	handler.ExchangeCurrency(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	if len(mockSvc.calls) != 0 {
		t.Fatalf("expected no service calls, got %v", mockSvc.calls)
	}
}

func TestHandler_ExchangeCurrency_ServiceUnavailable(t *testing.T) {
	mockSvc := &mockWalletService{
		exchangeFunc: func(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error {
			return status.Error(codes.Unavailable, "exchange service unavailable")
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"from_currency": "USD",
		"to_currency": "EUR",
		"amount": 100
	}`

	req := httptest.NewRequest(http.MethodPost, "/exchange", bytes.NewBufferString(body))

	const userID int64 = 42

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, userID),
	)

	rec := httptest.NewRecorder()

	handler.ExchangeCurrency(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	mockSvc.assertCalls(t, "Exchange")
}

func TestHandler_ExchangeCurrency_DeadlineExceeded(t *testing.T) {
	mockSvc := &mockWalletService{
		exchangeFunc: func(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error {
			return status.Error(codes.DeadlineExceeded, "exchange timeout")
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"from_currency": "USD",
		"to_currency": "EUR",
		"amount": 100
	}`

	req := httptest.NewRequest(http.MethodPost, "/exchange", bytes.NewBufferString(body))

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, int64(42)),
	)

	rec := httptest.NewRecorder()

	handler.ExchangeCurrency(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status %d, got %d", http.StatusGatewayTimeout, rec.Code)
	}

	mockSvc.assertCalls(t, "Exchange")
}

func TestHandler_ExchangeCurrency_InvalidArgument(t *testing.T) {
	mockSvc := &mockWalletService{
		exchangeFunc: func(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error {
			return status.Error(codes.InvalidArgument, "insufficient funds")
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"from_currency": "USD",
		"to_currency": "EUR",
		"amount": 100
	}`

	req := httptest.NewRequest(http.MethodPost, "/exchange", bytes.NewBufferString(body))

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, int64(42)),
	)

	rec := httptest.NewRecorder()

	handler.ExchangeCurrency(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	mockSvc.assertCalls(t, "Exchange")
}

func TestHandler_ExchangeCurrency_InternalError(t *testing.T) {
	mockSvc := &mockWalletService{
		exchangeFunc: func(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error {
			return errors.New("database error")
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"from_currency": "USD",
		"to_currency": "EUR",
		"amount": 100
	}`

	req := httptest.NewRequest(http.MethodPost, "/exchange", bytes.NewBufferString(body))

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, int64(42)),
	)

	rec := httptest.NewRecorder()

	handler.ExchangeCurrency(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	mockSvc.assertCalls(t, "Exchange")
}

func TestHandler_ExchangeCurrency_GetBalancesError(t *testing.T) {
	mockSvc := &mockWalletService{
		exchangeFunc: func(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error {
			return nil
		},

		getBalancesFunc: func(ctx context.Context, userID int64) (map[string]string, error) {
			return nil, errors.New("database error")
		},
	}

	handler := newHandler(mockSvc)

	body := `{
		"from_currency": "USD",
		"to_currency": "EUR",
		"amount": 100
	}`

	req := httptest.NewRequest(http.MethodPost, "/exchange", bytes.NewBufferString(body))

	req = req.WithContext(
		context.WithValue(req.Context(), UserIDKey, int64(42)),
	)

	rec := httptest.NewRecorder()

	handler.ExchangeCurrency(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	mockSvc.assertCalls(t, "Exchange", "GetBalances")
}

func TestHandler_ExchangeCurrency_TransactionErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		fromCurrency string
		toCurrency   string
		amount       float64
	}{
		{
			name:         "invalid amount",
			err:          service.ErrInvalidAmount,
			fromCurrency: "USD",
			toCurrency:   "EUR",
			amount:       -100,
		},
		{
			name:         "invalid currency",
			err:          service.ErrInvalidCurrency,
			fromCurrency: "XXX",
			toCurrency:   "EUR",
			amount:       100,
		},
		{
			name:         "same currency",
			err:          service.ErrSameCurrency,
			fromCurrency: "USD",
			toCurrency:   "USD",
			amount:       100,
		},
		{
			name:         "insufficient funds",
			err:          wallRep.ErrInsufficientFunds,
			fromCurrency: "USD",
			toCurrency:   "EUR",
			amount:       100000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &mockWalletService{
				exchangeFunc: func(ctx context.Context, userID int64, fromCur, toCur string, amount float64) error {
					return tt.err
				},
			}

			handler := newHandler(mockSvc)

			body := fmt.Sprintf(`{
				"from_currency": "%s",
				"to_currency": "%s",
				"amount": %f
			}`,
				tt.fromCurrency,
				tt.toCurrency,
				tt.amount,
			)

			req := httptest.NewRequest(http.MethodPost, "/exchange", bytes.NewBufferString(body))

			req = req.WithContext(
				context.WithValue(req.Context(), UserIDKey, int64(42)),
			)

			rec := httptest.NewRecorder()

			handler.ExchangeCurrency(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}

			mockSvc.assertCalls(t, "Exchange")

			var got map[string]string

			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if got["error"] != tt.err.Error() {
				t.Errorf("expected error %q, got %q", tt.err.Error(), got["error"])
			}
		})
	}
}

func TestHandler_GetRates_Success(t *testing.T) {
	expectedRates := map[string]float32{
		"USD": 1.0,
		"EUR": 0.92,
		"GBP": 0.79,
	}

	mockSvc := &mockWalletService{
		getRatesFunc: func(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error) {
			return &pb.ExchangeRatesResponse{Rates: expectedRates}, nil
		},
	}

	handler := newHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/rates", nil)
	rec := httptest.NewRecorder()

	handler.GetRates(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	mockSvc.assertCalls(t, "GetRates")

	var got GetRateResponse

	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !reflect.DeepEqual(got.Rates, expectedRates) {
		t.Errorf("expected rates %v, got %v", expectedRates, got.Rates)
	}
}

func TestHandler_GetRates_ServiceUnavailable(t *testing.T) {
	mockSvc := &mockWalletService{
		getRatesFunc: func(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error) {
			return nil, status.Error(codes.Unavailable, "exchange service unavailable")
		},
	}

	handler := newHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/rates", nil)
	rec := httptest.NewRecorder()

	handler.GetRates(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}

	mockSvc.assertCalls(t, "GetRates")
}

func TestHandler_GetRates_DeadlineExceeded(t *testing.T) {
	mockSvc := &mockWalletService{
		getRatesFunc: func(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error) {
			return nil, status.Error(codes.DeadlineExceeded, "exchange service timeout")
		},
	}

	handler := newHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/rates", nil)
	rec := httptest.NewRecorder()

	handler.GetRates(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status %d, got %d", http.StatusGatewayTimeout, rec.Code)
	}

	mockSvc.assertCalls(t, "GetRates")
}

func TestHandler_GetRates_InternalError(t *testing.T) {
	mockSvc := &mockWalletService{
		getRatesFunc: func(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error) {
			return nil, errors.New("database connection failed")
		},
	}

	handler := newHandler(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/rates", nil)
	rec := httptest.NewRecorder()

	handler.GetRates(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	mockSvc.assertCalls(t, "GetRates")
}
