package models

import (
	"errors"
	"time"
)

const LargeTransferThreshold = 30000

type Operation string

const (
	OperationDeposit  Operation = "deposit"
	OperationWithdraw Operation = "withdraw"
	OperationExchange Operation = "exchange"
)

var (
	ErrInvalidOperation = errors.New("invalid operation")
	ErrAmountTooSmall   = errors.New("amount is below large transfer threshold")
)

type Transaction struct {
	TransactionID string    `json:"transaction_id" bson:"transaction_id"`
	UserID        string    `json:"user_id" bson:"user_id"`
	Operation     Operation `json:"operation" bson:"operation"`

	Amount   float64 `json:"amount" bson:"amount"`
	Currency string  `json:"currency" bson:"currency"`

	FromCurrency string  `json:"from_currency,omitempty" bson:"from_currency,omitempty"`
	ToCurrency   string  `json:"to_currency,omitempty" bson:"to_currency,omitempty"`
	Rate         float64 `json:"rate,omitempty" bson:"rate,omitempty"`

	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

func (t Transaction) Validate() error {
	if t.TransactionID == "" {
		return errors.New("transaction_id is required")
	}

	if t.UserID == "" {
		return errors.New("user_id is required")
	}

	if !t.Operation.IsValid() {
		return ErrInvalidOperation
	}

	if t.Amount < LargeTransferThreshold {
		return ErrAmountTooSmall
	}

	if t.Currency == "" {
		return errors.New("currency is required")
	}

	return nil
}

func (o Operation) IsValid() bool {
	switch o {
	case OperationDeposit, OperationWithdraw, OperationExchange:
		return true
	default:
		return false
	}
}
