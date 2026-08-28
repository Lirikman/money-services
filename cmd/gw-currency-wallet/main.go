package main

import (
	"database/sql"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/rs/cors"

	_ "github.com/Lirikman/money_services/docs"
	c "github.com/Lirikman/money_services/internal/config"
	logger "github.com/Lirikman/money_services/internal/logger"
	service "github.com/Lirikman/money_services/services/gw-currency-wallet/app"
	delivery "github.com/Lirikman/money_services/services/gw-currency-wallet/delivery"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/kafka"
	repository "github.com/Lirikman/money_services/services/gw-currency-wallet/repository/postgres"
	transport "github.com/Lirikman/money_services/services/gw-currency-wallet/transport"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           Swagger Documentation API
// @version         1.0
// @description     API-service currency-wallet
// @host            localhost:8080
// @BasePath        /api/v1
// @schemes   	    http https
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

	// инициализируем логгер
	log := logger.NewLogger(c.GetEnv("LOG_LEVEL", "INFO"))
	log.Debug("Config file flag parsed", slog.String("path", *configPath))
	log.Info("Starting service Currency-wallet")

	// Чтение переменных окружения
	dbURL := c.GetEnv("DB_URL", "postgres://postgres:password@localhost:5432/postgres?sslmode=disable")
	grpcAddr := c.GetEnv("EXCHANGE_GRPC_ADDR", "localhost:50051")
	jwtSecret := c.GetEnv("JWT_SECRET", "super_puper_secret_key")
	migratePath := c.GetEnv("DB_MIGRATIONS", "file://migrations")
	kafkaBrokers := []string{c.GetEnv("KAFKA_BROKERS", "localhost:9092")}
	kafkaTopic := c.GetEnv("KAFKA_TOPIC", "large-transfers")

	// Подключение к PostgresQL
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Error("Failed to connect to db", slog.Any("error", err))
	}
	defer db.Close()

	// Применяем миграции
	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: "currency_wallet",
	})

	if err != nil {
		log.Error("Failed to create migration driver", "err", err)
	}

	m, err := migrate.NewWithDatabaseInstance(migratePath, "postgres", driver)

	if err != nil {
		log.Error("Failed to initialize the migrator", slog.Any("err", err))
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		// если схема уже актуальна
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("Database is up to date, no changes")
		} else {
			log.Error("Error while running migration", "err", err)
		}
	} else {
		log.Info("Migrations successfully applied")
	}

	// Инициализация gRPC клиента
	cacheTTL := 5 * time.Minute
	grpcClient, err := transport.NewCurrencyClient(grpcAddr, log, cacheTTL)
	if err != nil {
		log.Error("Failed to connect to gRPC server", slog.Any("error", err))
	}

	// Сборка слоев приложения
	repoWall := repository.NewPostgresWalletRepository(db)
	repoUsr := repository.NewPostgresUserRepository(db)
	writer := kafka.NewProducer(kafkaBrokers, kafkaTopic)
	svc := service.NewWalletService(repoWall, grpcClient, writer)
	usr := service.NewUserService(repoUsr, jwtSecret)
	h := delivery.NewHandler(svc, usr, log)

	// Маршрутизация стандартным net/http
	mux := http.NewServeMux()

	// Публичные эндпоинты
	mux.HandleFunc("POST /api/v1/register", h.Register)
	mux.HandleFunc("POST /api/v1/login", h.Login)

	// Защищенные эндпоинты через Middleware
	authHandler := delivery.AuthMiddleware(jwtSecret)

	mux.Handle("GET /api/v1/balance", authHandler(http.HandlerFunc(h.Balance)))
	mux.Handle("POST /api/v1/wallet/deposit", authHandler(http.HandlerFunc(h.Deposit)))
	mux.Handle("POST /api/v1/wallet/withdraw", authHandler(http.HandlerFunc(h.Withdraw)))
	mux.Handle("POST /api/v1/exchange", authHandler(http.HandlerFunc(h.ExchangeCurrency)))
	mux.Handle("GET /api/v1/exchange/rates", authHandler(http.HandlerFunc(h.GetRates)))

	// Регистрация Swagger UI
	mux.HandleFunc("/api/v1/swagger/", httpSwagger.WrapHandler)

	// Настройка политики разрешений
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://localhost:8080", "http://127.0.0.1:8080"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		Debug:            true,
	})
	handlerWithCORS := c.Handler(mux)

	log.Info("Server is running on port", slog.String("port", "8080"))
	if err := http.ListenAndServe(":8080", handlerWithCORS); err != nil {
		log.Error("Server stopped", slog.Any("err", err))
	}
}
