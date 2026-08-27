package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	c "github.com/Lirikman/money_services/internal/config"
	"github.com/Lirikman/money_services/internal/logger"
	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	"github.com/Lirikman/money_services/services/gw-exchanger/server"
	"github.com/Lirikman/money_services/services/gw-exchanger/storage/repository"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

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

	log := logger.NewLogger(c.GetEnv("LOG_LEVEL", "INFO"))

	log.Info("Starting service Exchanger")
	log.Debug("Config file flag parsed", slog.String("path", *configPath))

	// Подключение к PostgresQL
	host := c.GetEnv("DB_HOST", "localhost")
	port := c.GetEnv("DB_PORT", "5432")
	user := c.GetEnv("DB_USER", "postgres")
	pass := c.GetEnv("DB_PASSWORD", "secret")
	name := c.GetEnv("DB_NAME", "postgres")
	migratePath := c.GetEnv("DB_MIGRATIONS", "file://migrations")

	conn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port, user, pass, name)
	db, err := sql.Open("postgres", conn)
	if err != nil {
		log.Error("failed to connect to database", "err", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Error("database ping failed", "err", err)
	}
	// Применяем миграции
	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: "exchanger",
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
	// Инициализация репозитория (pеализация postgres)
	repo := repository.NewPostgresRepository(db)

	// Создание gRPC сервера
	grpcServer := grpc.NewServer()
	exchangerServer := server.NewExchangerServer(repo, log)
	pb.RegisterExchangeServiceServer(
		grpcServer,
		exchangerServer,
	)

	// Запуск gRPC сервера
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		slog.Error("failed to listen", "err", err)
	}

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
