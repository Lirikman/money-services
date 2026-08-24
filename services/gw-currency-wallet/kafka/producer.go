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
	writer *kafkago.Writer
}

// Создание нового продюсера
func NewProducer(brokers []string, topic string) pr.Producer {
	return &kafkaProducer{
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(brokers...),
			Topic:        topic,
			BatchSize:    100,
			BatchTimeout: 10 * time.Millisecond,
			RequiredAcks: kafkago.RequireAll,
			Balancer:     &kafkago.Murmur2Balancer{},
		},
	}
}

// Отправка сообщений в kafka
func (p *kafkaProducer) Send(ctx context.Context, transfer models.Transaction) error {
	data, err := json.Marshal(transfer)

	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx,
		kafkago.Message{Key: []byte(transfer.TransactionID), Value: data},
	)
}

// Закрытие соединения
func (p *kafkaProducer) Close() error {
	return p.writer.Close()
}
