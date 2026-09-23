package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/golang-migrate/migrate/v4/database/clickhouse"

	"github.com/Lirikman/money_services/services/gw-analytics/config"
	kafkaConsumer "github.com/Lirikman/money_services/services/gw-analytics/kafka"
	repo "github.com/Lirikman/money_services/services/gw-analytics/repository/clickhouse"
	"github.com/Lirikman/money_services/services/gw-analytics/service"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	cfg, log := config.LoadAnalyticsConfig()
	slog.SetDefault(log)

	log.Info("Starting service gw-analytics",
		slog.String("kafka_topic", cfg.KafkaTopic),
		slog.String("kafka_group", cfg.KafkaGroupID),
		slog.String("clickhouse", cfg.ClickAddr),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	repo, err := repo.NewClickHouse(cfg.ClickAddr, cfg.ClickDB, cfg.ClickUser, cfg.ClickPassword)
	if err != nil {
		log.Error("failed to create clickhouse repository", slog.Any("error", err))
		os.Exit(1)
	}

	dsn := fmt.Sprintf("clickhouse://%s:%s@%s/%s?x-multi-statement=true", cfg.ClickUser, cfg.ClickPassword, cfg.ClickAddr, cfg.ClickDB)
	m, err := migrate.New(cfg.MigratePath, dsn)
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
		cfg.KafkaBrokers,
		cfg.KafkaTopic,
		cfg.KafkaGroupID,
		analyticsService,
		log,
	)

	log.Info("gw-analytics started")

	if err := consumer.Run(ctx); err != nil {
		log.Error("consumer stopped", slog.Any("error", err))
	}

	log.Info("gw-analytics stopped")
}
