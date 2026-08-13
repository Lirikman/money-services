package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Wallet struct {
	UserID   int64   `json:"user_id"`
	Currency string  `json:"currency"` // USD, EUR, RUB
	Balance  float32 `json:"balance"`
}
