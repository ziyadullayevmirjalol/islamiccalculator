package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/diyorbek/islamiccalculator/internal/pkg/httpx"
)

// Pinger reports whether a dependency is reachable. *pgxpool.Pool satisfies it.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Health struct {
	db Pinger
}

func NewHealth(db Pinger) *Health {
	return &Health{db: db}
}

// Live reports process liveness.
func (h *Health) Live(w http.ResponseWriter, _ *http.Request) {
	httpx.Data(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Ready reports readiness: the database must answer a ping.
func (h *Health) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		httpx.Data(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "db": "unreachable"})
		return
	}
	httpx.Data(w, http.StatusOK, map[string]string{"status": "ok"})
}
