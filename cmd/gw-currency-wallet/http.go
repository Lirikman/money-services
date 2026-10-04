package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	service "github.com/Lirikman/money_services/services/gw-currency-wallet/app"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/config"
	delivery "github.com/Lirikman/money_services/services/gw-currency-wallet/delivery"
	repository "github.com/Lirikman/money_services/services/gw-currency-wallet/repository"
	postgres "github.com/Lirikman/money_services/services/gw-currency-wallet/repository/postgres"
	"github.com/Lirikman/money_services/services/gw-currency-wallet/transport"
	"github.com/rs/cors"
	httpSwagger "github.com/swaggo/http-swagger"
)

func NewHTTPHandler(
	cfg *config.WalletConfig,
	db *sql.DB,
	grpcClient *transport.CurrencyClient,
	writer repository.Producer,
	log *slog.Logger,
) http.Handler {

	repoWallet := postgres.NewPostgresWalletRepository(db)
	repoUser := postgres.NewPostgresUserRepository(db)
	walletService := service.NewWalletService(repoWallet, grpcClient, writer)
	userService := service.NewUserService(repoUser, cfg.SecretJWT)
	handler := delivery.NewHandler(walletService, userService, log)

	mux := http.NewServeMux()
	registerRoutes(mux, handler, cfg.SecretJWT)

	return withCORS(mux)
}

func registerRoutes(mux *http.ServeMux, h *delivery.Handler, jwtSecret string) {
	mux.HandleFunc("POST /api/v1/register", h.Register)
	mux.HandleFunc("POST /api/v1/login", h.Login)

	auth := delivery.AuthMiddleware(jwtSecret)

	mux.Handle("GET /api/v1/balance", auth(http.HandlerFunc(h.Balance)))
	mux.Handle("POST /api/v1/wallet/deposit", auth(http.HandlerFunc(h.Deposit)))
	mux.Handle("POST /api/v1/wallet/withdraw", auth(http.HandlerFunc(h.Withdraw)))
	mux.Handle("POST /api/v1/exchange", auth(http.HandlerFunc(h.ExchangeCurrency)))
	mux.Handle("GET /api/v1/exchange/rates", auth(http.HandlerFunc(h.GetRates)))
	mux.HandleFunc("/api/v1/swagger/", httpSwagger.WrapHandler)
}

func withCORS(handler http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://localhost:8080", "http://127.0.0.1:8080"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		Debug:            true,
	})

	return c.Handler(handler)
}

func NewHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func (a *App) Run(ctx context.Context) error {
	go a.runHTTPServer()
	<-ctx.Done()
	a.log.Info("Shutdown signal received")
	return a.shutdown()
}

func (a *App) runHTTPServer() {
	a.log.Info("Server is running", slog.String("port", "8080"))

	if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		a.log.Error("HTTP server failed", slog.Any("error", err))
	}
}

func (a *App) shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		a.log.Error("HTTP server forced to shutdown", slog.Any("error", err))
	} else {
		a.log.Info("HTTP server stopped gracefully")
	}

	if err := a.writer.Close(); err != nil {
		a.log.Error("Failed to close Kafka producer", slog.Any("error", err))
	} else {
		a.log.Info("Kafka producer closed")
	}

	if err := a.grpcClient.Close(); err != nil {
		a.log.Error("Failed to close gRPC client", slog.Any("error", err))
	} else {
		a.log.Info("gRPC client closed")
	}

	if err := a.db.Close(); err != nil {
		a.log.Error("Failed to close database", slog.Any("error", err))
	} else {
		a.log.Info("Database connection closed")
	}

	return nil
}
