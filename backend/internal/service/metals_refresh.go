package service

import (
	"context"
	"log/slog"
	"time"
)

type MetalPriceFetcher interface {
	FetchLatest(ctx context.Context) ([]MetalPrice, error)
}

type MetalPriceWriter interface {
	Insert(ctx context.Context, price MetalPrice) error
}

// MetalRefresher keeps the metal_prices cache fresh: one refresh at
// startup, then one per interval. Failures are logged and retried next
// tick — the app keeps serving the last stored price, whose fetched_at
// makes any staleness visible.
type MetalRefresher struct {
	fetcher  MetalPriceFetcher
	writer   MetalPriceWriter
	interval time.Duration
}

func NewMetalRefresher(fetcher MetalPriceFetcher, writer MetalPriceWriter, interval time.Duration) *MetalRefresher {
	return &MetalRefresher{fetcher: fetcher, writer: writer, interval: interval}
}

func (r *MetalRefresher) Run(ctx context.Context) {
	r.RefreshOnce(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.RefreshOnce(ctx)
		}
	}
}

// RefreshOnce fetches current prices and stores every one it got; a
// partial failure stores what succeeded.
func (r *MetalRefresher) RefreshOnce(ctx context.Context) {
	prices, err := r.fetcher.FetchLatest(ctx)
	if err != nil {
		slog.Error("metal price refresh failed", "err", err)
		return
	}
	for _, price := range prices {
		if err := r.writer.Insert(ctx, price); err != nil {
			slog.Error("store metal price", "metal", price.Metal, "err", err)
			continue
		}
		slog.Info("metal price refreshed", "metal", price.Metal,
			"price_per_gram", price.PricePerGram.String(), "currency", price.Currency)
	}
}
