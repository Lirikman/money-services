package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Lirikman/money_services/services/gw-notification/config"
	kafkaclient "github.com/Lirikman/money_services/services/gw-notification/kafka"
	repository "github.com/Lirikman/money_services/services/gw-notification/repository/mongo"
	"github.com/Lirikman/money_services/services/gw-notification/service"
)

func main() {
	cfg, log := config.LoadNotificationConfig()
	slog.SetDefault(log)

	log.Info("Starting service gw-notification")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	repo, err := repository.NewMongoRepository(ctx, cfg.UriDB, cfg.NameDB, cfg.CollectName)

	if err != nil {
		log.Error("failed to initialize mongodb", slog.Any("error", err))
		os.Exit(1)
	}

	consumer := kafkaclient.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroupID, log)

	svc := service.NewNotificationService(consumer, repo, log, cfg.BatchSize, cfg.BatchTimeout)

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
