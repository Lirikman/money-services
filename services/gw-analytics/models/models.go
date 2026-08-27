package models

import "time"

type TransactionEvent struct {
	TransactionID string    `json:"transaction_id"`
	UserID        string    `json:"user_id"`
	Operation     string    `json:"operation"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at"`

	Status     string `json:"status"`
	RetryCount int    `json:"retry_count"`
	Error      string `json:"error,omitempty"`
}
