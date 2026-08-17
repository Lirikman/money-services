package server

import (
	"context"
	"log/slog"
	"strings"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	"github.com/Lirikman/money_services/services/gw-exchanger/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExchangerServer struct {
	pb.UnimplementedExchangeServiceServer
	repo storage.Repository
	log  *slog.Logger
}

func NewExchangerServer(repo storage.Repository, log *slog.Logger) *ExchangerServer {
	return &ExchangerServer{
		repo: repo,
		log:  log,
	}
}

// Получение списка курса обмена всех валют
func (s *ExchangerServer) GetExchangeRates(
	ctx context.Context, _ *pb.Empty,
) (*pb.ExchangeRatesResponse, error) {
	s.log.Debug("fetching exchange rates from repository")

	rates, err := s.repo.GetRates(ctx)
	if err != nil {
		s.log.Error("exchange rates could not be obtained", slog.Any("err", err))
		return nil, status.Error(codes.Internal, "internal server error")
	}

	s.log.Info("exchange rates successfully obtained", slog.Int("count", len(rates)))

	return &pb.ExchangeRatesResponse{
		Rates: rates,
	}, nil
}

// Получение списка курса обмена для конкретной валюты
func (s *ExchangerServer) GetExchangeRateForCurrency(
	ctx context.Context,
	req *pb.CurrencyRequest,
) (*pb.ExchangeRateResponse, error) {
	s.log.Debug("fetching exchange rate currency from repository")

	from := strings.ToUpper(req.GetFromCurrency())
	to := strings.ToUpper(req.GetToCurrency())

	// Валидация поддерживаемых валют
	if !isValidCurrency(from) || !isValidCurrency(to) {
		s.log.Error("an unsupported currency was requested",
			slog.String("from", from),
			slog.String("to", to),
		)
		return nil, status.Error(codes.InvalidArgument, "supported currencies are USD, RUB, EUR")
	}

	// Если валюты одинаковые, курс всегда 1.0
	if from == to {
		s.log.Debug("sale and exchange currencies are the same")
		return &pb.ExchangeRateResponse{
			FromCurrency: from,
			ToCurrency:   to,
			Rate:         1.0,
		}, nil
	}

	// Получение курса из базы данных
	rate, err := s.repo.GetRateCurrency(ctx, from, to)
	if err != nil {
		s.log.Error("Unable to obtain currency exchange rate",
			slog.String("from", from),
			slog.String("to", to),
			slog.Any("err", err),
		)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	s.log.Info("exchange rate successfully obtained",
		slog.String("from", from),
		slog.String("to", to),
	)

	return &pb.ExchangeRateResponse{
		FromCurrency: from,
		ToCurrency:   to,
		Rate:         rate,
	}, nil
}

// Валидация поддерживаемых валют
func isValidCurrency(cur string) bool {
	return cur == "USD" || cur == "RUB" || cur == "EUR"
}
