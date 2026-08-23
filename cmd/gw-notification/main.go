package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	l "github.com/Lirikman/money_services/internal/logger"
	kafkaclient "github.com/Lirikman/money_services/services/gw-notification/kafka"
	repository "github.com/Lirikman/money_services/services/gw-notification/repository/mongo"
	"github.com/Lirikman/money_services/services/gw-notification/service"
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
	log := l.NewLogger(getEnv("LOG_LEVEL", "INFO"))
	log.Debug("Config file flag parsed", slog.String("path", *configPath))
	log.Info("Starting gw-notification", slog.String("service", "gw-notification"))

	// чтение переменных окружения
	dbURI := getEnv("MONGO_URI", "mongodb://localhost:27017")
	dbName := getEnv("MONGO_DATABASE", "notification")
	dbCollect := getEnv("MONGO_COLLECTION", "transactions")
	kafkaBrokers := []string{getEnv("KAFKA_BROKERS", "localhost:9092")}
	kafkaTopic := getEnv("KAFKA_TOPIC", "large-transfers")
	kafkaGroupID := getEnv("KAFKA_GROUP_ID", "gw-notification")
	batchSize := getEnvInt("BATCH_SIZE", 500)
	batchTimeout := getEnvDuration("BATCH_TIMEOUT", 100*time.Millisecond)

	// контекст прослышивания системных сигналов
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// создаём репозиторий
	repo, err := repository.NewMongoRepository(ctx, dbURI, dbName, dbCollect)

	if err != nil {
		log.Error("failed to initialize mongodb", slog.Any("error", err))
		os.Exit(1)
	}

	// Создаём консьюмера
	consumer := kafkaclient.NewConsumer(kafkaBrokers, kafkaTopic, kafkaGroupID)

	// Создаём сервис сохранения денежных переводов
	svc := service.NewNotificationService(consumer, repo, log, batchSize, batchTimeout)

	// Запускаем сервис
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

func getEnv(key, defaultValue string) string {
	val := os.Getenv(key)
	if val == "" {
		slog.Warn("Environment variable not set, using default",
			slog.String("variable", key),
			slog.String("default", defaultValue),
		)
		return defaultValue
	}
	return val
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		slog.Warn("Environment variable not set, using default",
			slog.String("variable", key),
			slog.Int("default", defaultValue),
		)
		return defaultValue
	}

	result, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return result
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		slog.Warn("Environment variable not set, using default",
			slog.String("variable", key),
			slog.Duration("default", defaultValue),
		)
		return defaultValue
	}

	result, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}

	return result
}
