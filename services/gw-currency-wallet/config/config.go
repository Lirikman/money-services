package config

import (
	"log/slog"

	g "github.com/Lirikman/money_services/pkg/getter"
	i "github.com/Lirikman/money_services/pkg/init"
	l "github.com/Lirikman/money_services/pkg/logger"
)

type WalletConfig struct {
	UrlDB             string
	AddrGrpc          string
	SecretJWT         string
	MigratePath       string
	NotificationTopic string
	AnalyticsTopic    string
	KafkaBrokers      []string
}

func LoadWalletConfig() (*WalletConfig, *slog.Logger) {
	i.BaseInit()
	log := l.NewLogger(g.GetEnv("LOG_LEVEL", "INFO"))
	log.Debug("Config file initialized")
	cfg := &WalletConfig{
		UrlDB:             g.GetEnv("DB_URL", "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"),
		AddrGrpc:          g.GetEnv("EXCHANGE_GRPC_ADDR", "localhost:50051"),
		SecretJWT:         g.GetEnv("JWT_SECRET", "super_puper_secret_key"),
		MigratePath:       g.GetEnv("DB_MIGRATIONS", "file://migrations"),
		NotificationTopic: g.GetEnv("KAFKA_NOTIFICATION_TOPIC", "large-transfers"),
		AnalyticsTopic:    g.GetEnv("KAFKA_ANALYTICS_TOPIC", "wallet-transactions"),
		KafkaBrokers:      []string{g.GetEnv("KAFKA_BROKERS", "localhost:9092")},
	}

	return cfg, log
}
