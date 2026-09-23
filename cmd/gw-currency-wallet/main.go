package main

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/rs/cors"

	_ "github.com/Lirikman/money_services/docs"
	service "github.com/Lirikman/money_services/services/gw-currency-wallet/app"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/config"
	delivery "github.com/Lirikman/money_services/services/gw-currency-wallet/delivery"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/kafka"
	repository "github.com/Lirikman/money_services/services/gw-currency-wallet/repository/postgres"
	transport "github.com/Lirikman/money_services/services/gw-currency-wallet/transport"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           Swagger Documentation API
// @version         1.0
// @description     API-service currency-wallet
// @host            localhost:8080
// @BasePath        /api/v1
// @schemes   	    http https
func main() {
	cfg, log := config.LoadWalletConfig()
	slog.SetDefault(log)

	log.Info("Starting service Currency-wallet")

	db, err := sql.Open("postgres", cfg.UrlDB)
	if err != nil {
		log.Error("Failed to connect to db", slog.Any("error", err))
	}

	defer func() {
		_ = db.Close()
	}()

	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: "currency_wallet",
	})
	if err != nil {
		log.Error("Failed to create migration driver", "err", err)
	}

	m, err := migrate.NewWithDatabaseInstance(cfg.MigratePath, "postgres", driver)
	if err != nil {
		log.Error("Failed to initialize the migrator", slog.Any("err", err))
	}

	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Error("Error closing migration source", "error", srcErr)
		}
		if dbErr != nil {
			log.Error("Error closing the database connection", "error", dbErr)
		}
	}()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("Database is up to date, no changes")
		} else {
			log.Error("Error while running migration", "err", err)
		}
	} else {
		log.Info("Migrations successfully applied")
	}

	cacheTTL := 5 * time.Minute
	grpcClient, err := transport.NewCurrencyClient(cfg.AddrGrpc, log, cacheTTL)
	if err != nil {
		log.Error("Failed to connect to gRPC server", slog.Any("error", err))
	}

	repoWall := repository.NewPostgresWalletRepository(db)
	repoUsr := repository.NewPostgresUserRepository(db)
	writer := kafka.NewProducer(cfg.KafkaBrokers, cfg.NotificationTopic, cfg.AnalyticsTopic)
	svc := service.NewWalletService(repoWall, grpcClient, writer)
	usr := service.NewUserService(repoUsr, cfg.SecretJWT)
	h := delivery.NewHandler(svc, usr, log)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/register", h.Register)
	mux.HandleFunc("POST /api/v1/login", h.Login)

	authHandler := delivery.AuthMiddleware(cfg.SecretJWT)

	mux.Handle("GET /api/v1/balance", authHandler(http.HandlerFunc(h.Balance)))
	mux.Handle("POST /api/v1/wallet/deposit", authHandler(http.HandlerFunc(h.Deposit)))
	mux.Handle("POST /api/v1/wallet/withdraw", authHandler(http.HandlerFunc(h.Withdraw)))
	mux.Handle("POST /api/v1/exchange", authHandler(http.HandlerFunc(h.ExchangeCurrency)))
	mux.Handle("GET /api/v1/exchange/rates", authHandler(http.HandlerFunc(h.GetRates)))
	mux.HandleFunc("/api/v1/swagger/", httpSwagger.WrapHandler)

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
