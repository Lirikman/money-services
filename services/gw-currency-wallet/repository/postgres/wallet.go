package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
)

type PostgresWalletRepository struct {
	db *sql.DB
}

func NewPostgresWalletRepository(db *sql.DB) *PostgresWalletRepository {
	return &PostgresWalletRepository{db: db}
}

// Получение баланса
func (r *PostgresWalletRepository) GetBalances(ctx context.Context, userID int64) (map[string]string, error) {
	query := `SELECT user_id, currency, balance FROM wallets WHERE user_id = $1`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query balances: %w", err)
	}
	defer rows.Close()

	var wallets []models.Wallet
	for rows.Next() {
		var w models.Wallet
		if err := rows.Scan(&w.UserID, &w.Currency, &w.Balance); err != nil {
			return nil, fmt.Errorf("failed to scan wallet row: %w", err)
		}
		wallets = append(wallets, w)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during wallet rows iteration: %w", err)
	}

	balances := make(map[string]string, len(wallets))
	for _, wal := range wallets {
		balances[wal.Currency] = fmt.Sprintf("%.2f", wal.Balance)
	}

	return balances, nil
}

// Пополнение баланса
func (r *PostgresWalletRepository) Deposit(ctx context.Context, userID int64, currency string, amount float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("wallet deposit: begin tx failed: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO wallets (user_id, currency, balance) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (user_id, currency) 
		DO UPDATE SET balance = wallets.balance + EXCLUDED.balance`

	_, err = tx.ExecContext(ctx, query, userID, currency, amount)
	if err != nil {
		return fmt.Errorf("wallet deposit: exec query failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("wallet deposit: commit tx failed: %w", err)
	}

	return nil
}

var ErrWalletNotFound = errors.New("wallet not found")
var ErrInsufficientFunds = errors.New("insufficient funds")

// Списание баланса
func (r *PostgresWalletRepository) Withdraw(ctx context.Context, userID int64, currency string, amount float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("wallet withdraw: begin tx failed: %w", err)
	}
	defer tx.Rollback()

	var balance float64
	// FOR UPDATE блокирует строку для предотвращения Race Condition
	query := `SELECT balance FROM wallets WHERE user_id = $1 AND currency = $2 FOR UPDATE`
	err = tx.QueryRowContext(ctx, query, userID, currency).Scan(&balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrWalletNotFound
		}
		return fmt.Errorf("wallet withdraw: get balance query failed: %w", err)
	}

	if balance < amount {
		return ErrInsufficientFunds
	}

	_, err = tx.ExecContext(ctx, `UPDATE wallets SET balance = balance - $1 WHERE user_id = $2 AND currency = $3`, amount, userID, currency)
	if err != nil {
		return fmt.Errorf("withdrawal of funds from the balance failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("wallet withdraw: commit tx failed: %w", err)
	}

	return nil
}

// Обмен валют
func (r *PostgresWalletRepository) Exchange(ctx context.Context, userID int64, from, to string, amount, targetAmount float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("wallet exchange: begin tx failed: %w", err)
	}
	defer tx.Rollback()

	var fromBalance float64
	err = tx.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE user_id = $1 AND currency = $2 FOR UPDATE`, userID, from).Scan(&fromBalance)
	if err != nil {
		return fmt.Errorf("wallet exchange:failed to query user balances: %w", err)
	}

	if fromBalance < amount {
		return ErrInsufficientFunds
	}

	// Списание
	_, err = tx.ExecContext(ctx, `UPDATE wallets SET balance = balance - $1 WHERE user_id = $2 AND currency = $3`, amount, userID, from)
	if err != nil {
		return fmt.Errorf("wallet exchange: currency debit failed: %w", err)
	}

	// Зачисление целевой валюты
	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallets (user_id, currency, balance) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (user_id, currency) 
		DO UPDATE SET balance = wallets.balance + EXCLUDED.balance`, userID, to, targetAmount)
	if err != nil {
		return fmt.Errorf("wallet exchange: currency crediting failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("wallet exchange: commit tx failed: %w", err)
	}

	return nil
}
