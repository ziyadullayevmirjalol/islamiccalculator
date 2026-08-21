package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbek/islamiccalculator/internal/config"
	"github.com/diyorbek/islamiccalculator/internal/handler"
	"github.com/diyorbek/islamiccalculator/internal/provider/metals"
	"github.com/diyorbek/islamiccalculator/internal/repository/postgres"
	"github.com/diyorbek/islamiccalculator/internal/server"
	"github.com/diyorbek/islamiccalculator/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(".env")
	if err != nil {
		return err
	}
	if cfg.Auth.JWTSecret == "" {
		return errors.New("JWT_SECRET must be set (see .env.example)")
	}

	setupLogger(cfg.App)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DB.DSN())
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		// Not fatal: the server starts and /readyz reports degraded until
		// the database comes up (matters for container start ordering).
		slog.Warn("database not reachable at startup", "err", err)
	}

	settingsRepo := postgres.NewSettings(pool)
	metalsRepo := postgres.NewMetals(pool)
	rulesRepo := postgres.NewLivestockRules(pool)
	screenerRepo := postgres.NewScreenerRules(pool)
	historyRepo := postgres.NewCalculations(pool)
	usersRepo := postgres.NewUsers(pool)
	refreshRepo := postgres.NewRefreshTokens(pool)

	authSvc := service.NewAuth(usersRepo, refreshRepo, service.AuthConfig{
		Secret:     cfg.Auth.JWTSecret,
		AccessTTL:  cfg.Auth.AccessTTL,
		RefreshTTL: cfg.Auth.RefreshTTL,
	})

	router := server.NewRouter(server.Handlers{
		Health:       handler.NewHealth(pool),
		Finance:      handler.NewFinance(service.NewFinance(historyRepo)),
		Zakat:        handler.NewZakat(service.NewZakat(settingsRepo, metalsRepo, rulesRepo, historyRepo, cfg.Metals.StaleAfter)),
		Invest:       handler.NewInvest(service.NewInvest(screenerRepo, historyRepo)),
		Rates:        handler.NewRates(service.NewRates(settingsRepo, metalsRepo, cfg.Metals.StaleAfter)),
		Reference:    handler.NewReference(service.NewReference(rulesRepo)),
		Auth:         handler.NewAuth(authSvc),
		History:      handler.NewHistory(service.NewHistory(historyRepo)),
		VerifyAccess: authSvc.VerifyAccess,
	}, server.Options{
		MaxBodyBytes:    cfg.HTTP.MaxBodyBytes,
		RateLimitPerMin: cfg.HTTP.RateLimitPerMin,
	})

	if cfg.Metals.APIKey != "" {
		refresher := service.NewMetalRefresher(
			metals.NewClient(cfg.Metals.APIURL, cfg.Metals.APIKey, cfg.Metals.Currency),
			metalsRepo,
			cfg.Metals.RefreshInterval,
		)
		go refresher.Run(ctx)
	} else {
		slog.Info("metal price auto-refresh disabled: METALS_API_KEY not set; serving stored prices with staleness flags")
	}

	return server.Run(ctx, cfg.HTTP, router)
}

func setupLogger(app config.App) {
	level := slog.LevelInfo
	switch strings.ToLower(app.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	var h slog.Handler
	if app.Env == "dev" {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(h))
}
