package config

import (
	"log/slog"

	g "github.com/Lirikman/money_services/pkg/getter"
	i "github.com/Lirikman/money_services/pkg/init"
	l "github.com/Lirikman/money_services/pkg/logger"
)

type ExchangerConfig struct {
	Host        string
	Port        string
	User        string
	Password    string
	NameDB      string
	MigratePath string
}

func LoadExchangerConfig() (*ExchangerConfig, *slog.Logger) {
	i.BaseInit()
	log := l.NewLogger(g.GetEnv("LOG_LEVEL", "INFO"))
	log.Debug("Config file initialized")
	cfg := &ExchangerConfig{
		Host:        g.GetEnv("DB_HOST", "localhost"),
		Port:        g.GetEnv("DB_PORT", "5432"),
		User:        g.GetEnv("DB_USER", "postgres"),
		Password:    g.GetEnv("DB_PASSWORD", "secret"),
		NameDB:      g.GetEnv("DB_NAME", "postgres"),
		MigratePath: g.GetEnv("DB_MIGRATIONS", "file://migrations"),
	}

	return cfg, log
}
