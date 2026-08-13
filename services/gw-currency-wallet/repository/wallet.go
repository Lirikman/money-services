package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
)

type Wallet struct {
	db  *sql.DB
	log *slog.Logger
}

func NewWallet(db *sql.DB, log *slog.Logger) *Wallet {
	return &Wallet{db: db, log: log}
}

// Создание пользователя
func (r *Wallet) CreateUser(ctx context.Context, username, email, passwordHash string) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("failed to begin transaction", slog.Any("error", err))
		return 0, err
	}

	defer tx.Rollback()

	var id int64
	queryCreate := `INSERT INTO users (username, email, password_hash) VALUES ($1, $2, $3) RETURNING id`
	err = r.db.QueryRowContext(ctx, queryCreate, username, email, passwordHash).Scan(&id)
	if err != nil {
		r.log.Error("failed to create user",
			slog.String("username", username),
			slog.String("email", email),
			slog.Any("error", err),
		)
		return 0, err
	}

	// Создание валютных кошельков
	queryWallet := `INSERT INTO wallets(user_id,currency, balance) VALUES ($1, 'USD', 0.00), ($1, 'EUR', 0.00), ($1, 'RUB', 0.00)`
	_, err = tx.ExecContext(ctx, queryWallet, id)
	if err != nil {
		r.log.Error("failed to create a currency wallet",
			slog.Int64("user_id", id),
			slog.Any("error", err),
		)
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("failed to commit transaction", slog.Any("error", err))
		return 0, err
	}

	r.log.Info("user and wallets created successfully",
		slog.Int64("user_id", id),
		slog.String("username", username),
	)

	return id, nil
}

// Получение пользователя по имени
func (r *Wallet) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, username, email, password_hash, created_at FROM users WHERE username = $1`
	err := r.db.QueryRowContext(ctx, query, username).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Info("user not found", slog.String("username", username))
			return nil, nil
		}
		r.log.Error("failed to get user by username",
			slog.String("username", username),
			slog.Any("error", err),
		)
		return nil, err
	}
	r.log.Debug("user found successfully",
		slog.Int64("user_id", user.ID),
		slog.String("username", username),
	)
	return user, nil
}

// Получение баланса пользователя
func (r *Wallet) GetBalances(ctx context.Context, userID int64) (map[string]string, error) {
	query := `SELECT user_id, currency, balance FROM wallets WHERE user_id = $1`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		r.log.Error("failed to query user balances",
			slog.Int64("user_id", userID),
			slog.Any("error", err),
		)
		return nil, err
	}
	defer rows.Close()

	var wallets []models.Wallet
	for rows.Next() {
		var w models.Wallet
		if err := rows.Scan(&w.UserID, &w.Currency, &w.Balance); err != nil {
			r.log.Error("failed to scan wallet row",
				slog.Int64("user_id", userID),
				slog.Any("error", err),
			)
			return nil, err
		}
		wallets = append(wallets, w)
	}
	if err := rows.Err(); err != nil {
		r.log.Error("error during wallet rows iteration",
			slog.Int64("username", userID),
			slog.Any("error", err),
		)
		return nil, err
	}
	r.log.Debug("fetched user balances",
		slog.Int64("user_id", userID),
		slog.Int("count", len(wallets)),
	)
	balances := make(map[string]string, len(wallets))
	for _, wal := range wallets {
		balances[wal.Currency] = fmt.Sprintf("%.2f", wal.Balance)
	}

	return balances, nil
}

// Пополнение баланса
func (r *Wallet) Deposit(ctx context.Context, userID int64, currency string, amount float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("failed to begin deposit transaction",
			slog.Int64("UserID", userID),
			slog.String("currency", currency),
			slog.Float64("amount", amount),
			slog.Any("error", err),
		)
		return err
	}
	defer tx.Rollback()

	queryWallet := `
		INSERT INTO wallets (user_id, currency, balance) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (user_id, currency) 
		DO UPDATE SET balance = wallets.balance + EXCLUDED.balance`

	_, err = tx.ExecContext(ctx, queryWallet, userID, currency, amount)
	if err != nil {
		r.log.Error("wallet balance replenishment failed",
			slog.Int64("UserID", userID),
			slog.String("currency", currency),
			slog.Float64("amount", amount),
			slog.Any("error", err),
		)
		return err
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("failed to commit deposit transaction",
			slog.Int64("UserID", userID),
			slog.String("currency", currency),
			slog.Float64("amount", amount),
			slog.Any("error", err),
		)
		return err
	}

	r.log.Info("deposit successful",
		slog.Int64("UserID", userID),
		slog.String("currency", currency),
		slog.Float64("amount", amount),
	)

	return nil
}

// Списание баланса
func (r *Wallet) Withdraw(ctx context.Context, userID int64, currency string, amount float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("failed to begin withdraw transaction",
			slog.Int64("UserID", userID),
			slog.String("currency", currency),
			slog.Float64("amount", amount),
			slog.Any("error", err),
		)
		return err
	}
	defer tx.Rollback()

	var balance float64
	// FOR UPDATE блокирует строку для предотвращения Race Condition
	err = tx.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE user_id = $1 AND currency = $2 FOR UPDATE`, userID, currency).Scan(&balance)
	if err != nil {
		r.log.Error("failed to get user balance",
			slog.Int64("UserID", userID),
			slog.Any("error", err),
		)
		return errors.New("wallet not found")
	}

	if balance < amount {
		r.log.Error("amount exceeds the balance",
			slog.Int64("UserID", userID),
			slog.String("currency", currency),
			slog.Float64("amount", amount),
			slog.Any("error", err),
		)
		return errors.New("insufficient funds")
	}

	_, err = tx.ExecContext(ctx, `UPDATE wallets SET balance = balance - $1 WHERE user_id = $2 AND currency = $3`, amount, userID, currency)
	if err != nil {
		r.log.Error("withdrawal of funds from the balance failed",
			slog.Int64("UserID", userID),
			slog.String("currency", currency),
			slog.Float64("amount", amount),
			slog.Any("error", err),
		)
		return err
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("failed to commit withdraw transaction",
			slog.Int64("UserID", userID),
			slog.String("currency", currency),
			slog.Float64("amount", amount),
			slog.Any("error", err),
		)
		return err
	}

	r.log.Info("withdraw successful",
		slog.Int64("UserID", userID),
		slog.String("currency", currency),
		slog.Float64("amount", amount),
	)

	return nil
}

// Обмен валют
func (r *Wallet) Exchange(ctx context.Context, userID int64, from, to string, amount, targetAmount float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.log.Error("failed to begin exchange transaction",
			slog.Int64("UserID", userID),
			slog.String("from", from),
			slog.String("to", to),
			slog.Float64("amount", amount),
			slog.Any("error", err),
		)
		return err
	}
	defer tx.Rollback()

	var fromBalance float64
	err = tx.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE user_id = $1 AND currency = $2 FOR UPDATE`, userID, from).Scan(&fromBalance)
	if err != nil {
		r.log.Error("failed to query user balances",
			slog.Int64("UserID", userID),
			slog.Any("error", err),
		)
		return errors.New("source wallet not found")
	}

	if fromBalance < amount {
		r.log.Error("amount exceeds the balance",
			slog.Int64("UserID", userID),
			slog.String("from", from),
			slog.String("to", to),
			slog.Float64("amount", amount),
			slog.Any("error", err),
		)
		return errors.New("insufficient funds")
	}

	// Списание
	_, err = tx.ExecContext(ctx, `UPDATE wallets SET balance = balance - $1 WHERE user_id = $2 AND currency = $3`, amount, userID, from)
	if err != nil {
		r.log.Error("currency debit failed",
			slog.Int64("UserID", userID),
			slog.String("from", from),
			slog.Float64("amount", amount),
			slog.Any("error", err),
		)
		return err
	}

	// Зачисление целевой валюты
	_, err = tx.ExecContext(ctx, `
		INSERT INTO wallets (user_id, currency, balance) 
		VALUES ($1, $2, $3) 
		ON CONFLICT (user_id, currency) 
		DO UPDATE SET balance = wallets.balance + EXCLUDED.balance`, userID, to, targetAmount)
	if err != nil {
		r.log.Error("currency crediting failed",
			slog.Int64("UserID", userID),
			slog.String("to", to),
			slog.Float64("amount", targetAmount),
			slog.Any("error", err),
		)
		return err
	}

	if err := tx.Commit(); err != nil {
		r.log.Error("failed to commit exchange transaction",
			slog.Int64("UserID", userID),
			slog.String("from", from),
			slog.String("to", to),
			slog.Float64("amount", amount),
			slog.Any("error", err),
		)
		return err
	}

	r.log.Info("exchange successful",
		slog.Int64("UserID", userID),
		slog.String("from", from),
		slog.String("to", to),
		slog.Float64("amount", amount),
	)

	return nil
}
