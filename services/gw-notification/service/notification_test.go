package service

import (
	"testing"

	"github.com/Lirikman/money_services/services/gw-notification/models"
)

func TestTransactionValidate(t *testing.T) {

	tests := []struct {
		name    string
		tx      models.Transaction
		wantErr bool
	}{
		{
			name: "valid deposit",
			tx: models.Transaction{
				TransactionID: "tx-1",
				UserID:        "user-1",
				Operation:     models.OperationDeposit,
				Amount:        30000,
				Currency:      "EUR",
			},
			wantErr: false,
		},
		{
			name: "valid withdraw",
			tx: models.Transaction{
				TransactionID: "tx-2",
				UserID:        "user-1",
				Operation:     models.OperationWithdraw,
				Amount:        50000,
				Currency:      "EUR",
			},
			wantErr: false,
		},
		{
			name: "valid exchange",
			tx: models.Transaction{
				TransactionID: "tx-3",
				UserID:        "user-1",
				Operation:     models.OperationExchange,
				Amount:        40000,
				Currency:      "EUR",
				FromCurrency:  "EUR",
				ToCurrency:    "USD",
				Rate:          1.17,
			},
			wantErr: false,
		},
		{
			name: "amount below threshold",
			tx: models.Transaction{
				TransactionID: "tx-4",
				UserID:        "user-1",
				Operation:     models.OperationDeposit,
				Amount:        29999,
				Currency:      "EUR",
			},
			wantErr: true,
		},
		{
			name: "invalid operation",
			tx: models.Transaction{
				TransactionID: "tx-5",
				UserID:        "user-1",
				Operation:     "transfer",
				Amount:        50000,
				Currency:      "EUR",
			},
			wantErr: true,
		},
		{
			name: "empty transaction id",
			tx: models.Transaction{
				UserID:    "user-1",
				Operation: models.OperationDeposit,
				Amount:    50000,
				Currency:  "EUR",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {

			err := tt.tx.Validate()

			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf(
					"unexpected error: %v",
					err,
				)
			}
		})
	}
}
