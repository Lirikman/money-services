package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/golang-migrate/migrate/v4/database/clickhouse"

	c "github.com/Lirikman/money_services/pkg/config"
	l "github.com/Lirikman/money_services/pkg/logger"
	kafkaConsumer "github.com/Lirikman/money_services/services/gw-analytics/kafka"
	repo "github.com/Lirikman/money_services/services/gw-analytics/repository/clickhouse"
	"github.com/Lirikman/money_services/services/gw-analytics/service"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
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

	log := l.NewLogger(c.GetEnv("LOG_LEVEL", "INFO"))
	log.Debug("Config file flag parsed", slog.String("path", *configPath))

	clickAddr := c.GetEnv("CLICKHOUSE_ADDR", "localhost:9000")
	clickDB := c.GetEnv("CLICKHOUSE_DB", "default")
	clickUser := c.GetEnv("CLICKHOUSE_USER", "default")
	clickPass := c.GetEnv("CLICKHOUSE_PASSWORD", "password")
	kafkaBrokers := []string{c.GetEnv("KAFKA_BROKERS", "localhost:9092")}
	kafkaTopic := c.GetEnv("KAFKA_TOPIC", "wallet-transactions")
	kafkaGroupID := c.GetEnv("KAFKA_GROUP_ID", "gw-analytics")
	migrationPath := c.GetEnv("DB_MIGRATIONS", "file://migrations")

	log.Info("Starting gw-analytics",
		slog.String("service", "gw-analytics"),
		slog.String("kafka_topic", kafkaTopic),
		slog.String("kafka_group", kafkaGroupID),
		slog.String("clickhouse", clickAddr),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	repo, err := repo.NewClickHouse(clickAddr, clickDB, clickUser, clickPass)
	if err != nil {
		log.Error("failed to create clickhouse repository", slog.Any("error", err))
		os.Exit(1)
	}

	dsn := fmt.Sprintf("clickhouse://%s:%s@%s/%s?x-multi-statement=true", clickUser, clickPass, clickAddr, clickDB)
	m, err := migrate.New(migrationPath, dsn)
	if err != nil {
		slog.Error("Failed to initialize the migrator", slog.Any("error", err))
		os.Exit(1)
	}

	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Error("Error closing migration source", "error", srcErr)
		}
		if dbErr != nil {
			log.Error("Error closing the database connection", "error", dbErr)
		}
	}()

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

	analyticsService := service.NewAnalyticsService(repo)

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
