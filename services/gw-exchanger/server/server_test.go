package server_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	server "github.com/Lirikman/money_services/services/gw-exchanger/server"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MockRepository - фейковый репозиторий для тестов
type MockRepository struct {
	GetRatesFunc        func(ctx context.Context) (map[string]float32, error)
	GetRateCurrencyFunc func(ctx context.Context, from_currency, to_currency string) (float32, error)
}

func (m *MockRepository) GetRates(ctx context.Context) (map[string]float32, error) {
	return m.GetRatesFunc(ctx)
}

func (m *MockRepository) GetRateCurrency(ctx context.Context, from_currency, to_currency string) (float32, error) {
	return m.GetRateCurrencyFunc(ctx, from_currency, to_currency)
}

func TestGetRates(t *testing.T) {
	tests := []struct {
		name           string
		req            *pb.Empty
		mockBehavior   func(m *MockRepository)
		expectedRates  map[string]float32
		expectedStatus codes.Code
	}{
		{
			name: "Successful getting exchange rates",
			req:  &pb.Empty{},
			mockBehavior: func(m *MockRepository) {
				m.GetRatesFunc = func(ctx context.Context) (map[string]float32, error) {
					return map[string]float32{"USD/RUB": 1, "USD/EUR": 2, "RUB/EUR": 3}, nil
				}
			},
			expectedRates:  map[string]float32{"USD/RUB": 1, "USD/EUR": 2, "RUB/EUR": 3},
			expectedStatus: codes.OK,
		},
		{
			name: "Failed request to receive currencies",
			req:  &pb.Empty{},
			mockBehavior: func(m *MockRepository) {
				m.GetRatesFunc = func(ctx context.Context) (map[string]float32, error) {
					return nil, errors.New("database failure")
				}
			},
			expectedRates:  nil,
			expectedStatus: codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockRepository{}
			tt.mockBehavior(mockRepo)
			discardHandler := slog.NewTextHandler(io.Discard, nil)
			testLogger := slog.New(discardHandler)
			srv := server.NewExchangerServer(mockRepo, testLogger)
			res, err := srv.GetExchangeRates(context.Background(), tt.req)
			if tt.expectedStatus == codes.OK {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRates, res.Rates)
			} else {
				assert.Error(t, err)
				st, ok := status.FromError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedStatus, st.Code())
			}
		})
	}
}

func TestGetRateCurrency(t *testing.T) {
	tests := []struct {
		name           string
		req            *pb.CurrencyRequest
		mockBehavior   func(m *MockRepository)
		expectedRate   float32
		expectedStatus codes.Code
	}{
		{
			name: "Success USD to RUB",
			req: &pb.CurrencyRequest{
				FromCurrency: "USD",
				ToCurrency:   "RUB",
			},
			mockBehavior: func(m *MockRepository) {
				m.GetRateCurrencyFunc = func(ctx context.Context, from_currency, to_currency string) (float32, error) {
					return 83.06, nil
				}
			},
			expectedRate:   83.06,
			expectedStatus: codes.OK,
		},
		{
			name: "Same currency returns 1.0 without DB call",
			req: &pb.CurrencyRequest{
				FromCurrency: "EUR",
				ToCurrency:   "EUR",
			},
			mockBehavior:   func(m *MockRepository) {}, // DB не должна вызываться
			expectedRate:   1.0,
			expectedStatus: codes.OK,
		},
		{
			name: "Invalid argument - missing currency",
			req: &pb.CurrencyRequest{
				FromCurrency: "TRY",
				ToCurrency:   "RUB",
			},
			mockBehavior:   func(m *MockRepository) {},
			expectedRate:   0,
			expectedStatus: codes.InvalidArgument,
		},
		{
			name: "Exchange rate request failed",
			req: &pb.CurrencyRequest{
				FromCurrency: "USD",
				ToCurrency:   "RUB",
			},
			mockBehavior: func(m *MockRepository) {
				m.GetRateCurrencyFunc = func(ctx context.Context, from_currency, to_currency string) (float32, error) {
					return 0, errors.New("database failure")
				}
			},
			expectedRate:   0,
			expectedStatus: codes.Internal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockRepository{}
			tt.mockBehavior(mockRepo)

			discardHandler := slog.NewTextHandler(io.Discard, nil)
			testLogger := slog.New(discardHandler)
			srv := server.NewExchangerServer(mockRepo, testLogger)
			res, err := srv.GetExchangeRateForCurrency(context.Background(), tt.req)

			if tt.expectedStatus == codes.OK {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRate, res.Rate)
			} else {
				assert.Error(t, err)
				st, ok := status.FromError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedStatus, st.Code())
			}
		})
	}
}
