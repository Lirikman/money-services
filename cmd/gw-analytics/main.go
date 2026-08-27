package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	c "github.com/Lirikman/money_services/internal/config"
	l "github.com/Lirikman/money_services/internal/logger"
	kafkaConsumer "github.com/Lirikman/money_services/services/gw-analytics/kafka"
	repo "github.com/Lirikman/money_services/services/gw-analytics/repository/clickhouse"
	"github.com/Lirikman/money_services/services/gw-analytics/service"
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

	// подклчаем логгер
	log := l.NewLogger(c.GetEnv("LOG_LEVEL", "INFO"))
	log.Debug("Config file flag parsed", slog.String("path", *configPath))
	log.Info("Starting gw-notification", slog.String("service", "gw-notification"))

	// чтение переменных окружения
	clickAddr := c.GetEnv("CLICKHOUSE_ADDR", "localhost:9000")
	clickDB := c.GetEnv("CLICKHOUSE_DB", "analytics")
	kafkaBrokers := []string{c.GetEnv("KAFKA_BROKERS", "localhost:9092")}
	kafkaTopic := c.GetEnv("KAFKA_TOPIC", "wallet-transactions")
	kafkaGroupID := c.GetEnv("KAFKA_GROUP_ID", "gw-analytics")
	// aggrInterval := c.GetEnvInt("AGGREGATION_INTERVAL_SECONDS", 60)

	log.Info("starting gw-analytics",
		slog.String("kafka_topic", kafkaTopic),
		slog.String("kafka_group", kafkaGroupID),
		slog.String("clickhouse", clickAddr),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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
