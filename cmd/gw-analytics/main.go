package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/golang-migrate/migrate/v4/database/clickhouse"

	c "github.com/Lirikman/money_services/internal/config"
	l "github.com/Lirikman/money_services/internal/logger"
	kafkaConsumer "github.com/Lirikman/money_services/services/gw-analytics/kafka"
	repo "github.com/Lirikman/money_services/services/gw-analytics/repository/clickhouse"
	"github.com/Lirikman/money_services/services/gw-analytics/service"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {

	// получение переменных окружения
	configPath := flag.String("c", "config.env", "path to configuration file")
	flag.Parse()

	if _, err := os.Stat(*configPath); err == nil {
		slog.Info("Loading environment variables from file", slog.String("file", *configPath))
		if err := godotenv.Load(*configPath); err != nil {
			slog.Error("Error loading configuration file", slog.Any("error", err))
			os.Exit(1)
		}
	} else {
		slog.Warn("Configuration file not found, using system environment variables", slog.String("file", *configPath))
	}

	// подключаем логгер
	log := l.NewLogger(c.GetEnv("LOG_LEVEL", "INFO"))
	log.Debug("Config file flag parsed", slog.String("path", *configPath))

	// чтение переменных окружения
	clickAddr := c.GetEnv("CLICKHOUSE_ADDR", "localhost:9000")
	clickDB := c.GetEnv("CLICKHOUSE_DB", "default")
	kafkaBrokers := []string{c.GetEnv("KAFKA_BROKERS", "localhost:9092")}
	kafkaTopic := c.GetEnv("KAFKA_TOPIC", "wallet-transactions")
	kafkaGroupID := c.GetEnv("KAFKA_GROUP_ID", "gw-analytics")
	migrationPath := c.GetEnv("DB_MIGRATIONS", "file://migrations")
	// aggrInterval := c.GetEnvInt("AGGREGATION_INTERVAL_SECONDS", 60)

	log.Info("Starting gw-analytics",
		slog.String("service", "gw-analytics"),
		slog.String("kafka_topic", kafkaTopic),
		slog.String("kafka_group", kafkaGroupID),
		slog.String("clickhouse", clickAddr),
	)

	// Отслеживаем системные сигналы
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Подключение к Clickhouse
	db, err := sql.Open("clickhouse", "clickhouse://localhost:9000/default")
	if err != nil {
		log.Error("database connection error", slog.Any("err", err))
	}
	// Создаем целевую БД, если её нет
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS analytics")
	if err != nil {
		log.Error("database creation error", slog.Any("err", err))
	}
	db.Close()

	// Создаём репозиторий
	repo, err := repo.NewClickHouse(clickAddr, clickDB)
	if err != nil {
		log.Error("failed to create clickhouse repository", slog.Any("error", err))
		os.Exit(1)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()

	if err := repo.Ping(pingCtx); err != nil {
		log.Error("clickhouse is unavailable", slog.Any("error", err))
		os.Exit(1)
	}

	// Применение миграций
	dsn := fmt.Sprintf("clickhouse://%s/%s?x-multi-statement=true", clickAddr, clickDB)
	m, err := migrate.New(migrationPath, dsn)
	if err != nil {
		slog.Error("Failed to initialize the migrator", slog.Any("error", err))
		os.Exit(1)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		// если схема уже актуальна
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("Database is up to date, no changes")
		} else {
			log.Error("Error while running migration", "err", err)
		}
	} else {
		log.Info("Migrations successfully applied")
	}

	// Создаём сервис аналитики
	analyticsService := service.NewAnalyticsService(repo)

	// Создаём consumer
	consumer := kafkaConsumer.NewConsumer(
		kafkaBrokers,
		kafkaTopic,
		kafkaGroupID,
		analyticsService,
		log,
	)

	log.Info("gw-analytics started")

	if err := consumer.Run(ctx); err != nil {
		log.Error("consumer stopped", slog.Any("error", err))
	}

	log.Info("gw-analytics stopped")
}
