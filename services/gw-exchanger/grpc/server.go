package server

import (
	"context"
	"log/slog"
	"strings"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	"github.com/Lirikman/money_services/services/gw-exchanger/app"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedExchangeServiceServer
	service *app.Service
	log     *slog.Logger
}

func New(service *app.Service, log *slog.Logger) *Server {
	return &Server{
		service: service,
		log:     log,
	}
}

// Получение списка курса обмена всех валют
func (s *Server) GetExchangeRates(
	ctx context.Context, _ *pb.Empty,
) (*pb.ExchangeRatesResponse, error) {
	rates, err := s.service.GetRateCurrency(ctx)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "exchange rates could not be obtained: %v", err)
	}

	return &pb.ExchangeRatesResponse{
		Rates: rates,
	}, nil
}

// Получение списка курса обмена для конкретной валюты
func (s *Server) GetExchangeRateForCurrency(
	ctx context.Context,
	req *pb.CurrencyRequest,
) (*pb.ExchangeRateResponse, error) {
	from := strings.ToUpper(req.GetFromCurrency())
	to := strings.ToUpper(req.GetToCurrency())

	// Валидация поддерживаемых валют
	if !isValidCurrency(from) || !isValidCurrency(to) {
		return nil, status.Error(codes.InvalidArgument, "supported currencies are USD, RUB, EUR")
	}

	// Если валюты одинаковые, курс всегда 1.0
	if from == to {
		return &pb.ExchangeRateResponse{
			FromCurrency: from,
			ToCurrency:   to,
			Rate:         1.0,
		}, nil
	}

	// Получение курса из базы данных
	rate, err := s.service.GetRate(ctx, from, to)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "exchange rate not found: %v", err)
	}

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
