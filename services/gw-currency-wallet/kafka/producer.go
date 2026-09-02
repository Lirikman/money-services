package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
	pr "github.com/Lirikman/money_services/services/gw-currency-wallet/repository"
	kafkago "github.com/segmentio/kafka-go"
)

type kafkaProducer struct {
	writer            *kafkago.Writer
	notificationTopic string
	analyticsTopic    string
}

// Создание нового продюсера
func NewProducer(brokers []string, notificationTopic, analyticsTopic string) pr.Producer {
	return &kafkaProducer{
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(brokers...),
			BatchSize:    100,
			BatchTimeout: 10 * time.Millisecond,
			RequiredAcks: kafkago.RequireAll,
			Balancer:     &kafkago.Murmur2Balancer{},
		},
		notificationTopic: notificationTopic,
		analyticsTopic:    analyticsTopic,
	}
}

// Отправка сообщений в kafka сервис Notification
func (p *kafkaProducer) SendNotification(ctx context.Context, transfer models.Transaction) error {
	data, err := json.Marshal(transfer)

	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx,
		kafkago.Message{Topic: p.notificationTopic, Key: []byte(transfer.TransactionID), Value: data},
	)
}

// Отправка сообщений в kafka сервис Analytics
func (p *kafkaProducer) SendAnalytics(ctx context.Context, analytics models.TransactionEvent) error {
	data, err := json.Marshal(analytics)

	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx,
		kafkago.Message{Topic: p.analyticsTopic, Key: []byte(analytics.TransactionID), Value: data},
	)
}

// Закрытие соединения
func (p *kafkaProducer) Close() error {
	return p.writer.Close()
}
