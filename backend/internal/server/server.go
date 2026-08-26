// Package server assembles the chi router, shared middleware, and the
// HTTP server lifecycle (start + graceful shutdown).
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"gopkg.in/yaml.v3"

	"github.com/diyorbek/islamiccalculator/internal/config"
	"github.com/diyorbek/islamiccalculator/internal/handler"
	"github.com/diyorbek/islamiccalculator/internal/openapi"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
	"github.com/diyorbek/islamiccalculator/internal/pkg/authctx"
	"github.com/diyorbek/islamiccalculator/internal/pkg/httpx"
)

// Handlers bundles the route handlers the router mounts. New calculator
// groups are added here phase by phase.
type Handlers struct {
	Health    *handler.Health
	Finance   *handler.Finance
	Zakat     *handler.Zakat
	Invest    *handler.Invest
	Rates     *handler.Rates
	Reference *handler.Reference
	Auth      *handler.Auth
	History   *handler.History

	// VerifyAccess validates a bearer token and returns the user ID.
	VerifyAccess func(token string) (string, error)
}

// Options are the router's hardening knobs; zero values disable a limit
// (useful in tests).
type Options struct {
	MaxBodyBytes    int64
	RateLimitPerMin int
}

// NewRouter wires middleware and routes. Calculator route groups mount
// under /api/v1 as their phases land.
func NewRouter(h Handlers, opts Options) http.Handler {
	r := chi.NewRouter()
	// Captured (not shadowed) for the export handler's in-process
	// self-dispatch: by the time any request arrives, every route below
	// is registered, so re-entering through `root` reaches the exact
	// same validation and calculation path a direct call would.
	root := r

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	if opts.RateLimitPerMin > 0 {
		r.Use(httprate.Limit(opts.RateLimitPerMin, time.Minute,
			httprate.WithKeyFuncs(httprate.KeyByIP),
			httprate.WithLimitHandler(func(w http.ResponseWriter, _ *http.Request) {
				httpx.Err(w, apperr.RateLimited("too many requests; slow down"))
			}),
		))
	}
	if opts.MaxBodyBytes > 0 {
		r.Use(maxBody(opts.MaxBodyBytes))
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodOptions},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
	}))

	r.Get("/healthz", h.Health.Live)
	r.Get("/readyz", h.Health.Ready)

	r.Route("/api/v1", func(r chi.Router) {
		// Optional auth everywhere: an authenticated calculation is saved
		// under its user; anonymous requests work unchanged.
		r.Use(optionalAuth(h.VerifyAccess))

		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", h.Auth.Register)
			r.Post("/login", h.Auth.Login)
			r.Post("/refresh", h.Auth.Refresh)
		})
		r.Route("/history", func(r chi.Router) {
			r.Use(requireUser)
			r.Get("/", h.History.List)
			r.Delete("/{id}", h.History.Delete)
		})
		r.Route("/finance", func(r chi.Router) {
			r.Post("/murabaha", h.Finance.Murabaha)
			r.Post("/ijara", h.Finance.Ijara)
			r.Post("/qard-hasan", h.Finance.QardHasan)
			r.Post("/mudaraba", h.Finance.Mudaraba)
			r.Post("/diminishing-musharaka", h.Finance.DimMusharaka)
			r.Post("/salam", h.Finance.Salam)
			r.Post("/istisna", h.Finance.Istisna)
			r.Post("/musharaka", h.Finance.Musharaka)
			r.Post("/late-payment", h.Finance.LatePayment)
		})
		r.Route("/zakat", func(r chi.Router) {
			r.Post("/wealth", h.Zakat.Wealth)
			r.Post("/business", h.Zakat.Business)
			r.Post("/ushr", h.Zakat.Ushr)
			r.Post("/livestock", h.Zakat.Livestock)
			r.Post("/fidya", h.Zakat.Fidya)
			r.Post("/fitrah", h.Zakat.Fitrah)
			r.Post("/tazkiya", h.Zakat.Tazkiya)
		})
		r.Route("/invest", func(r chi.Router) {
			r.Post("/screener", h.Invest.Screener)
			r.Post("/sukuk", h.Invest.Sukuk)
		})
		r.Route("/export", func(r chi.Router) {
			r.Post("/xlsx", handler.NewExport(root).Handle)
		})
		r.Route("/rates", func(r chi.Router) {
			r.Get("/metals", h.Rates.Metals)
		})
		r.Route("/reference", func(r chi.Router) {
			r.Get("/livestock-rules", h.Reference.LivestockRules)
		})
		r.Get("/docs", docsPage)
		r.Get("/docs/openapi.yaml", openAPISpec)
		r.Get("/docs/openapi.json", openAPISpecJSON)
	})

	return r
}

// maxBody caps request body size; oversized bodies surface as a typed
// validation error from httpx.Decode.
func maxBody(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

func openAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(openapi.Spec)
}

// specJSON is the embedded YAML contract converted once at first request,
// so the JSON form can never drift from the YAML source of truth.
var specJSON = sync.OnceValues(func() ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(openapi.Spec, &doc); err != nil {
		return nil, err
	}
	return json.MarshalIndent(doc, "", "  ")
})

func openAPISpecJSON(w http.ResponseWriter, _ *http.Request) {
	body, err := specJSON()
	if err != nil {
		httpx.Err(w, apperr.Internal("render openapi.json", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func docsPage(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<html>
<head>
  <title>Islamic Calculator API</title>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({ url: "/api/v1/docs/openapi.yaml", dom_id: "#swagger-ui" });
  </script>
</body>
</html>`))
}

// optionalAuth resolves a Bearer token into the request context when one
// is presented. A missing header passes through untouched (calculators
// are anonymous-first); a presented-but-invalid token is a hard 401 so
// clients notice expired sessions instead of silently losing history.
func optionalAuth(verify func(string) (string, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				next.ServeHTTP(w, r)
				return
			}
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				httpx.Err(w, apperr.Unauthorized("authorization header must be: Bearer <token>"))
				return
			}
			userID, err := verify(token)
			if err != nil {
				httpx.Err(w, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(authctx.WithUserID(r.Context(), userID)))
		})
	}
}

func requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authctx.UserID(r.Context()); !ok {
			httpx.Err(w, apperr.Unauthorized("authentication required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration", time.Since(start).String(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}

// Run starts the HTTP server and blocks until ctx is canceled, then
// shuts down gracefully within the configured timeout.
func Run(ctx context.Context, cfg config.HTTP, h http.Handler) error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      h,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		slog.Info("shutting down http server")
		return srv.Shutdown(shutdownCtx)
	}
}
