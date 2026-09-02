package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Wallet struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Currency  string    `json:"currency"` // USD, EUR, RUB
	Balance   float32   `json:"balance"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Transaction struct {
	TransactionID string    `json:"transaction_id" bson:"transaction_id"`
	UserID        string    `json:"user_id" bson:"user_id"`
	Operation     string    `json:"operation" bson:"operation"` // deposit, withdraw, exchange
	Amount        float64   `json:"amount" bson:"amount"`
	Currency      string    `json:"currency" bson:"currency"`
	FromCurrency  string    `json:"from_currency,omitempty" bson:"from_currency,omitempty"`
	ToCurrency    string    `json:"to_currency,omitempty" bson:"to_currency,omitempty"`
	Rate          float64   `json:"rate,omitempty" bson:"rate,omitempty"`
	CreatedAt     time.Time `json:"created_at" bson:"created_at"`
}

type TransactionEvent struct {
	TransactionID string    `json:"transaction_id"`
	UserID        string    `json:"user_id"`
	Operation     string    `json:"operation"`
	CreatedAt     time.Time `json:"created_at"`
	Status        string    `json:"status"`
	RetryCount    int       `json:"retry_count"`
	Error         string    `json:"error,omitempty"`
}
