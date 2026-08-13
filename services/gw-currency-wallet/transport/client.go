package client

import (
	"context"
	"log/slog"
	"sync"
	"time"

	pb "github.com/Lirikman/money_services/proto-exchange/generate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CurrencyRateCache struct {
	Rate      float64
	UpdatedAt time.Time
}

type CurrencyClient struct {
	client pb.ExchangeServiceClient
	log    *slog.Logger

	// Конфигурация кэша
	cacheTTL time.Duration
	mu       sync.RWMutex

	// Кэш для метода GetRates
	ratesCache     *pb.ExchangeRatesResponse
	ratesCacheTime time.Time

	// Кэш для метода GetRateCurrency
	currencyCache map[string]CurrencyRateCache
}

func NewCurrencyClient(addr string, log *slog.Logger, cacheTTL time.Duration) (*CurrencyClient, error) {
	log.Info("initializing grpc currency client")

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Error("failed to establish grpc connection", slog.Any("error", err))
		return nil, err
	}
	log.Info("grpc currency client successfully created")
	return &CurrencyClient{
		client:        pb.NewExchangeServiceClient(conn),
		log:           log,
		cacheTTL:      cacheTTL,
		currencyCache: make(map[string]CurrencyRateCache),
	}, nil
}

func (c *CurrencyClient) GetRates(ctx context.Context, req *pb.Empty) (*pb.ExchangeRatesResponse, error) {
	// Проверка актуальности кэша под RLock
	c.mu.RLock()
	hasCache := c.ratesCache != nil
	isCacheValid := time.Since(c.ratesCacheTime) < c.cacheTTL
	if hasCache && isCacheValid {
		c.log.Debug("returning all exchange rates from local cache")
		cachedResp := c.ratesCache
		c.mu.RUnlock()
		return cachedResp, nil
	}
	c.mu.RUnlock()

	// Запрос к серверу
	c.log.Debug("sending grpc request to get exchange rates")
	resp, err := c.client.GetExchangeRates(ctx, req)
	if err != nil {
		c.log.Error("failed to get exchange rates via grpc", slog.Any("error", err))
		// Fallback: возврат устаревших данных при ошибке
		c.mu.RLock()
		if c.ratesCache != nil {
			c.log.Warn("grpc failed, returning stale rates cache as fallback")
			staleResp := c.ratesCache
			c.mu.RUnlock()
			return staleResp, nil
		}
		c.mu.RUnlock()
		return nil, err
	}

	// Обновление кэша под Lock
	c.mu.Lock()
	c.ratesCache = resp
	c.ratesCacheTime = time.Now()
	c.mu.Unlock()

	c.log.Info("successfully fetched exchange rates via grpc")
	return resp, nil
}

func (c *CurrencyClient) GetRateCurrency(ctx context.Context, from, to string) (float64, error) {
	cacheKey := from + "/" + to

	// Проверка актуальности кэша в map под RLock
	c.mu.RLock()
	cachedData, exists := c.currencyCache[cacheKey]
	isCacheValid := time.Since(cachedData.UpdatedAt) < c.cacheTTL
	if exists && isCacheValid {
		c.log.Debug("returning specific currency rate from local cache", slog.String("pair", cacheKey))
		rate := cachedData.Rate
		c.mu.RUnlock()
		return rate, nil
	}
	c.mu.RUnlock()

	// Запрос к серверу
	c.log.Debug("sending grpc request for specific currency rate")
	resp, err := c.client.GetExchangeRateForCurrency(ctx, &pb.CurrencyRequest{
		FromCurrency: from,
		ToCurrency:   to,
	})
	if err != nil {
		c.log.Error("failed to get currency exchange rate via grpc",
			slog.String("from", from),
			slog.String("to", to),
			slog.Any("error", err),
		)
		// Fallback: возврат устаревшего одиночного курса при ошибке
		c.mu.RLock()
		if exists {
			c.log.Warn("grpc failed, returning stale currency cache as fallback", slog.String("pair", cacheKey))
			staleRate := cachedData.Rate
			c.mu.RUnlock()
			return staleRate, nil
		}
		c.mu.RUnlock()

		return 0, err
	}

	actualRate := float64(resp.Rate)

	// Обновление карты под Lock
	c.mu.Lock()
	c.currencyCache[cacheKey] = CurrencyRateCache{
		Rate:      actualRate,
		UpdatedAt: time.Now(),
	}
	c.mu.Unlock()

	c.log.Info("successfully fetched currency exchange rate")
	return float64(resp.Rate), nil
}
