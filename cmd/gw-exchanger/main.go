package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	"github.com/Lirikman/money_services/services/gw-exchanger/config"
	"github.com/Lirikman/money_services/services/gw-exchanger/server"
	"github.com/Lirikman/money_services/services/gw-exchanger/storage/repository"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	cfg, log := config.LoadExchangerConfig()
	slog.SetDefault(log)

	log.Info("Starting service Exchanger")

	conn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.NameDB)
	db, err := sql.Open("postgres", conn)
	if err != nil {
		log.Error("failed to connect to database", "err", err)
	}

	defer func() {
		_ = db.Close()
	}()

	if err := db.Ping(); err != nil {
		log.Error("database ping failed", slog.Any("err", err))
		os.Exit(1)
	}

	log.Info("PostgreSQL connection established")

	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: "exchanger",
	})

	if err != nil {
		log.Error("Failed to create migration driver", "err", err)
	}

	m, err := migrate.NewWithDatabaseInstance(cfg.MigratePath, "postgres", driver)

	if err != nil {
		log.Error("Failed to initialize the migrator", "err", err)
		os.Exit(1)
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
			os.Exit(1)
		}
	} else {
		log.Info("Migrations successfully applied")
	}

	repo := repository.NewPostgresRepository(db)

	grpcServer := grpc.NewServer()
	exchangerServer := server.NewExchangerServer(repo, log)
	pb.RegisterExchangeServiceServer(
		grpcServer,
		exchangerServer,
	)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(
		grpcServer,
		healthServer,
	)
	healthServer.SetServingStatus(
		"",
		grpc_health_v1.HealthCheckResponse_NOT_SERVING,
	)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		slog.Error("failed to listen", "err", err)
		os.Exit(1)
	}

	healthServer.SetServingStatus(
		"",
		grpc_health_v1.HealthCheckResponse_SERVING,
	)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("gRPC server started", slog.String("addr", lis.Addr().String()))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc server stopped", slog.Any("err", err))
		}
	}()

	sig := <-sigChan

	log.Info("Received signal shutting down gracefully", slog.String("sig", sig.String()))

	healthServer.SetServingStatus(
		"",
		grpc_health_v1.HealthCheckResponse_NOT_SERVING,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stopped := make(chan struct{})

	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-ctx.Done():
		log.Info("Graceful shutdown timed out, forcing stop...")
		grpcServer.Stop()
	case <-stopped:
		log.Info("Server gracefully stopped")
	}
}
