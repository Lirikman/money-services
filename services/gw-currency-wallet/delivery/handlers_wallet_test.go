package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
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
