package kafka

import (
	"context"
	"encoding/json"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/Lirikman/money_services/services/gw-currency-wallet/models"
)

type Producer struct {
	writer *kafkago.Writer
}

// Создание нового продюсера
func NewProducer(brokers []string, topic string) *Producer {
	return &Producer{
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(brokers...),
			Topic:        topic,
			BatchSize:    100,
			BatchTimeout: 10 * time.Millisecond,
			RequiredAcks: kafkago.RequireAll,
		},
	}
}

// Отправка сообщений в kafka
func (p *Producer) Send(ctx context.Context, transfer models.Transaction) error {
	data, err := json.Marshal(transfer)

	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx,
		kafkago.Message{Key: []byte(transfer.TransactionID), Value: data},
	)
}

// Закрытие соединения
func (p *Producer) Close() error {
	return p.writer.Close()
}
