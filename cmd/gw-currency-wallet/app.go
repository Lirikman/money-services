package main

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Lirikman/money_services/services/gw-currency-wallet/config"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/kafka"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/repository"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/transport"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type App struct {
	server     *http.Server
	db         *sql.DB
	grpcClient *transport.CurrencyClient
	writer     repository.Producer
	log        *slog.Logger
}

func NewApp(cfg *config.WalletConfig, log *slog.Logger) (*App, error) {

	db, err := initDB(cfg, log)
	if err != nil {
		return nil, err
	}

	if err := runMigrations(db, cfg, log); err != nil {
		_ = db.Close()
		return nil, err
	}

	grpcClient, err := transport.NewCurrencyClient(cfg.AddrGrpc, log, 5*time.Minute)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	writer := kafka.NewProducer(cfg.KafkaBrokers, cfg.NotificationTopic, cfg.AnalyticsTopic)
	handler := NewHTTPHandler(cfg, db, grpcClient, writer, log)
	server := NewHTTPServer(handler)

	return &App{
		server:     server,
		db:         db,
		grpcClient: grpcClient,
		writer:     writer,
		log:        log,
	}, nil
}

func initDB(cfg *config.WalletConfig, log *slog.Logger) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.UrlDB)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	log.Info("Database connection established")

	return db, nil
}

func runMigrations(db *sql.DB, cfg *config.WalletConfig, log *slog.Logger) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{MigrationsTable: "currency_wallet"})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(cfg.MigratePath, "postgres", driver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("Database is up to date, no changes")
			return nil
		}
		return err
	}

	log.Info("Migrations successfully applied")

	return nil
}
