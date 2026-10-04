package main

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/Lirikman/money_services/services/gw-currency-wallet/config"

	_ "github.com/Lirikman/money_services/docs"
)

// @title           Swagger Documentation API
// @version         1.0
// @description     API-service currency-wallet
// @host            localhost:8080
// @BasePath        /api/v1
// @schemes         http https

func main() {
	cfg, log := config.LoadWalletConfig()
	slog.SetDefault(log)

	log.Info("Starting service Currency-wallet")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := NewApp(cfg, log)
	if err != nil {
		log.Error("Failed to initialize application", slog.Any("error", err))
		return
	}

	if err := app.Run(ctx); err != nil {
		log.Error("Application stopped with error", slog.Any("error", err))
		return
	}

	log.Info("Currency-wallet stopped gracefully")
}
