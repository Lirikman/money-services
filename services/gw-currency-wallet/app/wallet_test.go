package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	service "github.com/Lirikman/money_services/services/gw-currency-wallet/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock для gRPC клиента
type mockCurrencyClient struct {
	mock.Mock
	pb.ExchangeServiceClient
	err error
}

func (m *mockCurrencyClient) GetRates(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	rate := map[string]float32{"USD/RUB": 1, "USD/EUR": 2, "RUB/EUR": 3}
	return &pb.ExchangeRatesResponse{Rates: rate}, nil
}

func (m *mockCurrencyClient) GetRateCurrency(ctx context.Context, from, to string) (float64, error) {
	args := m.Called(ctx, from, to)
	return args.Get(0).(float64), args.Error(1)
}

// Mock для репозитория
type mockWalletRepo struct {
	mock.Mock
	balances map[string]string
	err      error
}

func (m *mockWalletRepo) Deposit(ctx context.Context, userID int64, currency string, amount float64) error {
	args := m.Called(ctx, userID, currency, amount)
	return args.Error(0)

}
func (m *mockWalletRepo) Withdraw(ctx context.Context, userID int64, currency string, amount float64) error {
	args := m.Called(ctx, userID, currency, amount)
	return args.Error(0)
}
func (m *mockWalletRepo) Exchange(ctx context.Context, userID int64, fromCur, toCur string, amount, targetAmount float64) error {
	args := m.Called(ctx, userID, fromCur, toCur, amount, targetAmount)
	return args.Error(0)
}
func (m *mockWalletRepo) GetBalances(ctx context.Context, userID int64) (map[string]string, error) {
	return m.balances, m.err
}

func TestGetRate_Success(t *testing.T) {
	repo := &mockWalletRepo{}
	gClient := &mockCurrencyClient{}

	svc := service.NewWalletService(repo, gClient)
	rate, err := svc.GetRates(context.Background(), &pb.Empty{})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rate.Rates["USD/RUB"] != 1 || rate.Rates["USD/EUR"] != 2 || rate.Rates["RUB/EUR"] != 3 {
		t.Errorf(`expected rate {"USD/RUB": 1, "USD/EUR": 2, "RUB/EUR": 3}, got %v`, rate.Rates)
	}
}

func TestGetRate_Error(t *testing.T) {
	repo := &mockWalletRepo{}
	gClient := &mockCurrencyClient{err: errors.New("grpc error")}

	svc := service.NewWalletService(repo, gClient)
	_, err := svc.GetRates(context.Background(), &pb.Empty{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetBalances(t *testing.T) {
	mockData := map[string]string{"USD": "1000", "RUB": "2000", "EUR": "1500"}
	repo := &mockWalletRepo{balances: mockData}
	gClient := &mockCurrencyClient{}

	svc := service.NewWalletService(repo, gClient)
	res, err := svc.GetBalances(context.Background(), 1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res) != 3 || res["USD"] != "1000" || res["RUB"] != "2000" || res["EUR"] != "1500" {
		t.Errorf("unexpected output: %v", res)
	}
}

func TestWalletService_Deposit(t *testing.T) {

	errInternal := fmt.Errorf("Internal server error")

	type fields struct {
		mockRepoFn func(m *mockWalletRepo)
	}
	type args struct {
		userID   int64
		currency string
		amount   float64
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr error
	}{
		{
			name: "Amount is less than or equal to zero",
			fields: fields{
				mockRepoFn: func(m *mockWalletRepo) {},
			},
			args: args{
				userID:   1,
				currency: "USD",
				amount:   0,
			},
			wantErr: service.ErrInvalidAmount,
		},
		{
			name: "Unsupported currency",
			fields: fields{
				mockRepoFn: func(m *mockWalletRepo) {},
			},
			args: args{
				userID:   1,
				currency: "INVALID",
				amount:   100.0,
			},
			wantErr: service.ErrInvalidCurrency,
		},
		{
			name: "Repository failure",
			fields: fields{
				mockRepoFn: func(m *mockWalletRepo) {
					m.On("Deposit", mock.Anything, int64(1), "USD", 100.0).
						Return(errInternal)
				},
			},
			args: args{
				userID:   1,
				currency: "USD",
				amount:   100.0,
			},
			wantErr: errInternal,
		},
		{
			name: "Deposit completed",
			fields: fields{
				mockRepoFn: func(m *mockWalletRepo) {
					m.On("Deposit", mock.Anything, int64(1), "USD", 100.0).
						Return(nil)
				},
			},
			args: args{
				userID:   1,
				currency: "USD",
				amount:   100.0,
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mockWalletRepo)
			tt.fields.mockRepoFn(mockRepo)

			s := service.NewWalletService(mockRepo, &mockCurrencyClient{})

			err := s.Deposit(context.Background(), tt.args.userID, tt.args.currency, tt.args.amount)

			assert.ErrorIs(t, err, tt.wantErr)

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestWalletService_Withdraw(t *testing.T) {
	type fields struct {
		mockRepoFn func(m *mockWalletRepo)
	}
	type args struct {
		userID   int64
		currency string
		amount   float64
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr error
	}{
		{
			name: "Amount is less than or equal to zero",
			fields: fields{
				mockRepoFn: func(m *mockWalletRepo) {},
			},
			args: args{
				userID:   1,
				currency: "RUB",
				amount:   -100,
			},
			wantErr: service.ErrInvalidAmount,
		},
		{
			name: "Unsupported currency",
			fields: fields{
				mockRepoFn: func(m *mockWalletRepo) {},
			},
			args: args{
				userID:   1,
				currency: "INVALID",
				amount:   100.0,
			},
			wantErr: service.ErrInvalidCurrency,
		},
		{
			name: "Repository failure",
			fields: fields{
				mockRepoFn: func(m *mockWalletRepo) {
					m.On("Withdraw", mock.Anything, int64(1), "RUB", 500.0).
						Return(errors.New("insufficient funds"))
				},
			},
			args: args{
				userID:   1,
				currency: "RUB",
				amount:   500.0,
			},
			wantErr: errors.New("insufficient funds"),
		},
		{
			name: "Withdraw completed",
			fields: fields{
				mockRepoFn: func(m *mockWalletRepo) {
					m.On("Withdraw", mock.Anything, int64(1), "RUB", 50.0).
						Return(nil)
				},
			},
			args: args{
				userID:   1,
				currency: "RUB",
				amount:   50.0,
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mockWalletRepo)
			tt.fields.mockRepoFn(mockRepo)

			s := service.NewWalletService(mockRepo, &mockCurrencyClient{})

			err := s.Withdraw(context.Background(), tt.args.userID, tt.args.currency, tt.args.amount)

			if tt.wantErr != nil {
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestWalletService_Exchange(t *testing.T) {
	var (
		ErrGrpcInternal = errors.New("grpc connection error")
		ErrRepoInternal = errors.New("tx failed")
	)

	// Структура для настройки mock-объектов в каждом тесте
	type fields struct {
		mockClientFn func(m *mockCurrencyClient)
		mockRepoFn   func(m *mockWalletRepo)
	}
	type args struct {
		userID int64
		from   string
		to     string
		amount float64
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr error
	}{
		{
			name: "Amount is less than or equal to zero",
			fields: fields{
				mockClientFn: func(m *mockCurrencyClient) {},
				mockRepoFn:   func(m *mockWalletRepo) {},
			},
			args:    args{userID: 1, from: "USD", to: "EUR", amount: 0},
			wantErr: service.ErrInvalidAmount,
		},
		{
			name: "Unsupported currency",
			fields: fields{
				mockClientFn: func(m *mockCurrencyClient) {},
				mockRepoFn:   func(m *mockWalletRepo) {},
			},
			// Предполагаем, что обе валюты не пройдут isSupportedCurrency
			args:    args{userID: 1, from: "INVALID1", to: "INVALID2", amount: 100.0},
			wantErr: service.ErrInvalidCurrency,
		},
		{
			name: "Same currencies for exchange",
			fields: fields{
				mockClientFn: func(m *mockCurrencyClient) {},
				mockRepoFn:   func(m *mockWalletRepo) {},
			},
			args:    args{userID: 1, from: "USD", to: "USD", amount: 100.0},
			wantErr: service.ErrSameCurrency,
		},
		{
			name: "Error requesting course via gRPC",
			fields: fields{
				mockClientFn: func(m *mockCurrencyClient) {
					m.On("GetRateCurrency", mock.Anything, "USD", "RUB").
						Return(0.0, ErrGrpcInternal) // gRPC возвращает ошибку
				},
				mockRepoFn: func(m *mockWalletRepo) {}, // До репозитория код не дойдет
			},
			args:    args{userID: 1, from: "USD", to: "RUB", amount: 100.0},
			wantErr: ErrGrpcInternal,
		},
		{
			name: "Repository failure during exchange",
			fields: fields{
				mockClientFn: func(m *mockCurrencyClient) {
					m.On("GetRateCurrency", mock.Anything, "USD", "EUR").
						Return(0.85, nil) // Курс 0.85
				},
				mockRepoFn: func(m *mockWalletRepo) {
					// 100 USD * 0.85 = 85.0 EUR. Проверяем, что передается верный targetAmount
					m.On("Exchange", mock.Anything, int64(1), "USD", "EUR", 100.0, 85.0).
						Return(ErrRepoInternal)
				},
			},
			args:    args{userID: 1, from: "USD", to: "EUR", amount: 100.0},
			wantErr: ErrRepoInternal,
		},
		{
			name: "Rate has been received, exchange has been completed",
			fields: fields{
				mockClientFn: func(m *mockCurrencyClient) {
					m.On("GetRateCurrency", mock.Anything, "USD", "EUR").
						Return(0.85, nil)
				},
				mockRepoFn: func(m *mockWalletRepo) {
					m.On("Exchange", mock.Anything, int64(1), "USD", "EUR", 100.0, 85.0).
						Return(nil)
				},
			},
			args:    args{userID: 1, from: "USD", to: "EUR", amount: 100.0},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(mockCurrencyClient)
			mockRepo := new(mockWalletRepo)

			tt.fields.mockClientFn(mockClient)
			tt.fields.mockRepoFn(mockRepo)

			// Инициализируем сервис обеими заглушками
			s := service.NewWalletService(mockRepo, mockClient)

			err := s.Exchange(context.Background(), tt.args.userID, tt.args.from, tt.args.to, tt.args.amount)

			// Проверка ожидаемой ошибки
			assert.ErrorIs(t, err, tt.wantErr)

			// Проверяем, что все запланированные вызовы mock-методов состоялись
			mockClient.AssertExpectations(t)
			mockRepo.AssertExpectations(t)
		})
	}
}
