package config

import (
	"log/slog"
	"time"

	g "github.com/Lirikman/money_services/pkg/getter"
	i "github.com/Lirikman/money_services/pkg/init"
	l "github.com/Lirikman/money_services/pkg/logger"
)

type NotificationConfig struct {
	UriDB        string
	NameDB       string
	CollectName  string
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string
	BatchSize    int
	BatchTimeout time.Duration
}

func LoadNotificationConfig() (*NotificationConfig, *slog.Logger) {
	i.BaseInit()
	log := l.NewLogger(g.GetEnv("LOG_LEVEL", "INFO"))
	log.Debug("Config file initialized")
	cfg := &NotificationConfig{
		UriDB:        g.GetEnv("MONGO_URI", "mongodb://root:secret@localhost:27017/mydb?authSource=admin"),
		NameDB:       g.GetEnv("MONGO_DATABASE", "notification"),
		CollectName:  g.GetEnv("MONGO_COLLECTION", "transactions"),
		KafkaBrokers: []string{g.GetEnv("KAFKA_BROKERS", "localhost:9092")},
		KafkaTopic:   g.GetEnv("KAFKA_TOPIC", "large-transfers"),
		KafkaGroupID: g.GetEnv("KAFKA_GROUP_ID", "gw-notification"),
		BatchSize:    g.GetEnvInt("BATCH_SIZE", 500),
		BatchTimeout: g.GetEnvDuration("BATCH_TIMEOUT", 500*time.Millisecond),
	}

	return cfg, log
}
