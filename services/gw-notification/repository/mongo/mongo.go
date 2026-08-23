package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/Lirikman/money_services/services/gw-notification/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrDuplicate = errors.New("duplicate transaction")

type MongoRepository struct {
	client     *mongo.Client
	collection *mongo.Collection
}

// Создание нового репозитория mongo_db
// Подключение к БД
func NewMongoRepository(ctx context.Context, uri string, database string, collection string) (*MongoRepository, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))

	if err != nil {
		return nil, fmt.Errorf("connect mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	coll := client.
		Database(database).
		Collection(collection)

	// создание уникального индекса
	_, err = coll.Indexes().CreateOne(ctx,
		mongo.IndexModel{
			Keys:    bson.D{{Key: "transaction_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	)

	if err != nil {
		return nil, fmt.Errorf("create index: %w", err)
	}

	return &MongoRepository{client: client, collection: coll}, nil
}

// Сохранение денежного перевода
func (r *MongoRepository) Save(ctx context.Context, transaction models.Transaction) error {
	_, err := r.collection.InsertOne(ctx, transaction)

	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("insert transaction: %w", err)
	}
	return nil
}

// массовое сохранение денежных переводов
func (r *MongoRepository) SaveBatch(ctx context.Context, transactions []models.Transaction) error {
	if len(transactions) == 0 {
		return nil
	}

	models := make([]mongo.WriteModel, 0, len(transactions))

	for _, transaction := range transactions {
		models = append(models, mongo.NewInsertOneModel().SetDocument(transaction))
	}

	_, err := r.collection.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))

	if err == nil {
		return nil
	}

	// Duplicate documents считаем успешно обработанными
	var bulkErr mongo.BulkWriteException

	if errors.As(err, &bulkErr) {
		for _, writeErr := range bulkErr.WriteErrors {
			if writeErr.Code != 11000 {
				return fmt.Errorf("bulk insert: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("bulk insert: %w", err)
}

// Закрытие соединения mongo_db
func (r *MongoRepository) Close(ctx context.Context) error {
	return r.client.Disconnect(ctx)
}
