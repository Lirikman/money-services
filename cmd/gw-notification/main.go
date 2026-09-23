package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	c "github.com/Lirikman/money_services/pkg/config"
	l "github.com/Lirikman/money_services/pkg/logger"
	kafkaclient "github.com/Lirikman/money_services/services/gw-notification/kafka"
	repository "github.com/Lirikman/money_services/services/gw-notification/repository/mongo"
	"github.com/Lirikman/money_services/services/gw-notification/service"
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
	log.Info("Starting gw-notification", slog.String("service", "gw-notification"))

	dbURI := c.GetEnv("MONGO_URI", "mongodb://root:secret@localhost:27017/mydb?authSource=admin")
	dbName := c.GetEnv("MONGO_DATABASE", "notification")
	dbCollect := c.GetEnv("MONGO_COLLECTION", "transactions")
	kafkaBrokers := []string{c.GetEnv("KAFKA_BROKERS", "localhost:9092")}
	kafkaTopic := c.GetEnv("KAFKA_TOPIC", "large-transfers")
	kafkaGroupID := c.GetEnv("KAFKA_GROUP_ID", "gw-notification")
	batchSize := c.GetEnvInt("BATCH_SIZE", 500)
	batchTimeout := c.GetEnvDuration("BATCH_TIMEOUT", 500*time.Millisecond)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	repo, err := repository.NewMongoRepository(ctx, dbURI, dbName, dbCollect)

	if err != nil {
		log.Error("failed to initialize mongodb", slog.Any("error", err))
		os.Exit(1)
	}

	consumer := kafkaclient.NewConsumer(kafkaBrokers, kafkaTopic, kafkaGroupID, log)

	svc := service.NewNotificationService(consumer, repo, log, batchSize, batchTimeout)

	err = svc.Run(ctx)

	if err != nil && !errors.Is(err, context.Canceled) {
		log.Error("service stopped with error", slog.Any("error", err))
	}

	log.Info("starting graceful shutdown")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := consumer.Close(); err != nil {
		log.Error("failed to close kafka consumer", slog.Any("error", err))
	}

	if err := repo.Close(shutdownCtx); err != nil {
		log.Error("failed to close mongodb", slog.Any("error", err))
	}

	log.Info("gw-notification stopped")
}
