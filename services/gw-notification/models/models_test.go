package models

import (
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestTransaction_Validate(t *testing.T) {
	// Базовая транзакция, удовлетворяющая всем правилам
	validTx := Transaction{
		TransactionID: "tx-123",
		UserID:        "user-789",
		Operation:     OperationDeposit,
		Amount:        35000.0,
		Currency:      "USD",
		CreatedAt:     time.Now(),
	}

	tests := []struct {
		name    string
		modify  func(tx Transaction) Transaction
		wantErr error
	}{
		{
			name: "Valid transaction",
			modify: func(tx Transaction) Transaction {
				return tx
			},
			wantErr: nil,
		},
		{
			name: "Missing TransactionID",
			modify: func(tx Transaction) Transaction {
				tx.TransactionID = ""
				return tx
			},
			wantErr: errors.New("transaction_id is required"),
		},
		{
			name: "Missing UserID",
			modify: func(tx Transaction) Transaction {
				tx.UserID = ""
				return tx
			},
			wantErr: errors.New("user_id is required"),
		},
		{
			name: "Invalid Operation",
			modify: func(tx Transaction) Transaction {
				tx.Operation = Operation("invalid_op")
				return tx
			},
			wantErr: ErrInvalidOperation,
		},
		{
			name: "Amount exactly at threshold",
			modify: func(tx Transaction) Transaction {
				tx.Amount = 30000.0
				return tx
			},
			wantErr: nil,
		},
		{
			name: "Amount below threshold",
			modify: func(tx Transaction) Transaction {
				tx.Amount = 29999.99
				return tx
			},
			wantErr: ErrAmountTooSmall,
		},
		{
			name: "Missing Currency",
			modify: func(tx Transaction) Transaction {
				tx.Currency = ""
				return tx
			},
			wantErr: errors.New("currency is required"),
		},
		{
			name: "Valid Exchange operation",
			modify: func(tx Transaction) Transaction {
				tx.Operation = OperationExchange
				tx.FromCurrency = "USD"
				tx.ToCurrency = "EUR"
				d128, _ := primitive.ParseDecimal128("0.92")
				tx.Rate = d128
				return tx
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := tt.modify(validTx)
			err := tx.Validate()

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error '%v', got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr.Error() {
					t.Errorf("expected error '%v', got '%v'", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got '%v'", err)
				}
			}
		})
	}
}
