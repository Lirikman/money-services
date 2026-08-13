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
	logger "github.com/Lirikman/money_services/internal/logger"
	service "github.com/Lirikman/money_services/services/gw-currency-wallet/app"
	delivery "github.com/Lirikman/money_services/services/gw-currency-wallet/delivery"
	repository "github.com/Lirikman/money_services/services/gw-currency-wallet/repository"
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

	// инициализируем логгер
	log := logger.New()
	log.Info("Starting service Currency-wallet")
	log.Debug("Config file flag parsed", slog.String("path", *configPath))

	if _, err := os.Stat(*configPath); err == nil {
		log.Info("Loading environment variables from file", slog.String("file", *configPath))
		if err := godotenv.Load(*configPath); err != nil {
			log.Error("Error loading configuration file", slog.Any("error", err))
			os.Exit(1)
		}
	} else {
		log.Warn("Configuration file not found, using system environment variables", slog.String("file", *configPath))
	}

	// Чтение переменных окружения
	dbURL := getEnv("DB_URL", "postgres://postgres:password@localhost:5432/postgres?sslmode=disable")
	grpcAddr := getEnv("EXCHANGE_GRPC_ADDR", "localhost:50051")
	jwtSecret := getEnv("JWT_SECRET", "super_puper_secret_key")
	migratePath := getEnv("DB_MIGRATIONS", "file://migrations")

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
	m, err := migrate.NewWithDatabaseInstance(
		migratePath,
		"postgres",
		driver,
	)
	if err != nil {
		log.Error("Failed to initialize the migrator", "err", err)
	}
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
	repo := repository.NewWallet(db, log)
	svc := service.NewWalletService(repo, grpcClient, jwtSecret)
	h := delivery.NewHandler(svc, log)

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
