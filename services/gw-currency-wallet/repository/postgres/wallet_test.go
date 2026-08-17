package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	repository "github.com/Lirikman/money_services/services/gw-currency-wallet/repository/postgres"
)

func TestPostgresRepo_Deposit_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open a stub database connection: %s", err)
	}
	defer db.Close()

	repo := repository.NewPostgresWalletRepository(db)

	userID := 1
	currency := "USD"
	amount := 150.50

	// Начало транзакции
	mock.ExpectBegin()
	// Ожидаем выполнение INSERT ... ON CONFLICT DO UPDATE
	mock.ExpectExec("INSERT INTO wallets").
		WithArgs(userID, currency, amount).
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Коммит
	mock.ExpectCommit()

	err = repo.Deposit(context.Background(), int64(userID), currency, amount)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestPostgresRepo_Withdraw_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open a stub database connection: %s", err)
	}
	defer db.Close()

	repo := repository.NewPostgresWalletRepository(db)

	userID := 1
	currency := "USD"
	amount := 50.0
	currentBalance := 100.0

	// Начало транзакции
	mock.ExpectBegin()

	// Проверка баланса с блокировкой FOR UPDATE
	rows := sqlmock.NewRows([]string{"balance"}).AddRow(currentBalance)
	mock.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(userID, currency).
		WillReturnRows(rows)

	// Обновление баланса
	mock.ExpectExec("UPDATE wallets SET balance = balance - \\$1").
		WithArgs(amount, userID, currency).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Коммит
	mock.ExpectCommit()

	err = repo.Withdraw(context.Background(), int64(userID), currency, amount)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestPostgresRepo_Withdraw_InsufficientFunds(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open a stub database connection: %s", err)
	}
	defer db.Close()

	repo := repository.NewPostgresWalletRepository(db)

	userID := 1
	currency := "USD"
	amount := 200.0
	currentBalance := 100.0 // Баланс меньше, чем запрашиваемая сумма

	mock.ExpectBegin()

	rows := sqlmock.NewRows([]string{"balance"}).AddRow(currentBalance)
	mock.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(userID, currency).
		WillReturnRows(rows)

	// Ожидаем откат транзакции (Rollback вызывается по defer)
	mock.ExpectRollback()

	err = repo.Withdraw(context.Background(), int64(userID), currency, amount)
	if err == nil {
		t.Error("expected error 'insufficient funds")
	} else if err.Error() != "insufficient funds" {
		t.Errorf("unexpected error message: %s", err.Error())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestPostgresRepo_ExchangeBalances_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open a stub database connection: %s", err)
	}
	defer db.Close()

	repo := repository.NewPostgresWalletRepository(db)

	userID := 1
	fromCurrency := "USD"
	toCurrency := "EUR"
	amount := 100.0
	rate := 0.92
	targetAmount := amount * rate // 92.0
	currentFromBalance := 150.0

	mock.ExpectBegin()

	// Блокировка и получение баланса исходной валюты
	rows := sqlmock.NewRows([]string{"balance"}).AddRow(currentFromBalance)
	mock.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(userID, fromCurrency).
		WillReturnRows(rows)

	// Списание с исходного кошелька
	mock.ExpectExec("UPDATE wallets SET balance = balance - \\$1").
		WithArgs(amount, userID, fromCurrency).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Зачисление (или создание) на целевой кошелек
	mock.ExpectExec("INSERT INTO wallets").
		WithArgs(userID, toCurrency, targetAmount).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	err = repo.Exchange(context.Background(), int64(userID), fromCurrency, toCurrency, amount, targetAmount)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestPostgresRepo_ExchangeBalances_DatabaseError_Rollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open a stub database connection: %s", err)
	}
	defer db.Close()

	repo := repository.NewPostgresWalletRepository(db)

	userID := 1
	fromCurrency := "USD"
	toCurrency := "EUR"
	amount := 100.0
	rate := 0.92
	targetAmount := amount * rate // 92.0
	currentFromBalance := 150.0

	mock.ExpectBegin()

	rows := sqlmock.NewRows([]string{"balance"}).AddRow(currentFromBalance)
	mock.ExpectQuery("SELECT balance FROM wallets").
		WithArgs(userID, fromCurrency).
		WillReturnRows(rows)

	// Симулируем падение базы данных на этапе списания средств
	mock.ExpectExec("UPDATE wallets SET balance = balance - \\$1").
		WithArgs(amount, userID, fromCurrency).
		WillReturnResult(sqlmock.NewErrorResult(errors.New("deadlock detected")))

	mock.ExpectRollback()

	err = repo.Exchange(context.Background(), int64(userID), fromCurrency, toCurrency, amount, targetAmount)
	if err == nil {
		t.Error("expected database error, got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
