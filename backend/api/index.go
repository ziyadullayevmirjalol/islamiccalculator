// Package api is the Vercel serverless entrypoint. Vercel invokes
// Handler per request; vercel.json rewrites every path here. The full
// chi router is built once per instance (cold start) and reused.
//
// Differences from cmd/server (the long-running deployment):
//   - no ListenAndServe, no graceful shutdown — Vercel owns the socket
//   - tiny DB pool (serverless instances multiply; Supabase pooler
//     handles the real pooling)
//   - no background metal-price refresher (no resident process)
package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/diyorbek/islamiccalculator/internal/config"
	"github.com/diyorbek/islamiccalculator/internal/handler"
	"github.com/diyorbek/islamiccalculator/internal/repository/postgres"
	"github.com/diyorbek/islamiccalculator/internal/server"
	"github.com/diyorbek/islamiccalculator/internal/service"
)

var (
	initOnce sync.Once
	router   http.Handler
	initErr  error
)

// Handler is the exported Vercel function entrypoint.
func Handler(w http.ResponseWriter, r *http.Request) {
	initOnce.Do(initialize)
	if initErr != nil {
		slog.Error("startup failed", "err", initErr)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"INTERNAL","message":"server misconfigured — check function logs"}}`))
		return
	}
	router.ServeHTTP(w, r)
}

func initialize() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load(".env") // no .env on Vercel: pure env vars
	if err != nil {
		initErr = err
		return
	}
	if cfg.Auth.JWTSecret == "" {
		initErr = errJWTSecret
		return
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DB.DSN())
	if err != nil {
		initErr = err
		return
	}
	poolCfg.MaxConns = 2 // per serverless instance; Supavisor does the real pooling
	poolCfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		initErr = err
		return
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

	router = server.NewRouter(server.Handlers{
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
}

type constErr string

func (e constErr) Error() string { return string(e) }

const errJWTSecret = constErr("JWT_SECRET must be set in the Vercel project environment variables")
