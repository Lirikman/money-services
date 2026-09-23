package config

import (
	"log/slog"

	g "github.com/Lirikman/money_services/pkg/getter"
	i "github.com/Lirikman/money_services/pkg/init"
	l "github.com/Lirikman/money_services/pkg/logger"
)

type AnalyticsConfig struct {
	ClickAddr     string
	ClickDB       string
	ClickUser     string
	ClickPassword string
	KafkaBrokers  []string
	KafkaTopic    string
	KafkaGroupID  string
	MigratePath   string
}

func LoadAnalyticsConfig() (*AnalyticsConfig, *slog.Logger) {
	i.BaseInit()
	log := l.NewLogger(g.GetEnv("LOG_LEVEL", "INFO"))
	log.Debug("Config file initialized")
	cfg := &AnalyticsConfig{
		ClickAddr:     g.GetEnv("CLICKHOUSE_ADDR", "localhost:9000"),
		ClickDB:       g.GetEnv("CLICKHOUSE_DB", "default"),
		ClickUser:     g.GetEnv("CLICKHOUSE_USER", "default"),
		ClickPassword: g.GetEnv("CLICKHOUSE_PASSWORD", "password"),
		KafkaBrokers:  []string{g.GetEnv("KAFKA_BROKERS", "localhost:9092")},
		KafkaTopic:    g.GetEnv("KAFKA_TOPIC", "wallet-transactions"),
		KafkaGroupID:  g.GetEnv("KAFKA_GROUP_ID", "gw-analytics"),
		MigratePath:   g.GetEnv("DB_MIGRATIONS", "file://migrations"),
	}

	return cfg, log
}
