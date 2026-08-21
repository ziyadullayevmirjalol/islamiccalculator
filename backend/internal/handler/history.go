package handler

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
	"github.com/diyorbek/islamiccalculator/internal/pkg/authctx"
	"github.com/diyorbek/islamiccalculator/internal/pkg/httpx"
	"github.com/diyorbek/islamiccalculator/internal/service"
)

type HistoryService interface {
	List(ctx context.Context, userID string, limit int, calcType string) ([]service.HistoryEntry, error)
	Delete(ctx context.Context, userID, id string) error
}

type History struct {
	svc HistoryService
}

func NewHistory(svc HistoryService) *History {
	return &History{svc: svc}
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type historyEntryDTO struct {
	ID        string `json:"id"`
	CalcType  string `json:"calcType"`
	Inputs    any    `json:"inputs"`
	Result    any    `json:"result"`
	CreatedAt string `json:"createdAt"`
}

func (h *History) List(w http.ResponseWriter, r *http.Request) {
	userID, _ := authctx.UserID(r.Context()) // guaranteed by requireUser middleware

	limit := 0
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			httpx.Err(w, apperr.Validation("limit must be a number",
				map[string]string{"limit": "must_be_integer"}))
			return
		}
		limit = n
	}

	entries, err := h.svc.List(r.Context(), userID, limit, r.URL.Query().Get("calcType"))
	if err != nil {
		httpx.Err(w, err)
		return
	}

	dtos := make([]historyEntryDTO, len(entries))
	for i, e := range entries {
		dtos[i] = historyEntryDTO{
			ID:        e.ID,
			CalcType:  e.CalcType,
			Inputs:    e.Inputs,
			Result:    e.Result,
			CreatedAt: e.CreatedAt.UTC().Format(time.RFC3339),
		}
	}
	httpx.Data(w, http.StatusOK, map[string]any{"entries": dtos})
}

func (h *History) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := authctx.UserID(r.Context())

	id := chi.URLParam(r, "id")
	if !uuidPattern.MatchString(id) {
		httpx.Err(w, apperr.NotFound("calculation not found"))
		return
	}
	if err := h.svc.Delete(r.Context(), userID, id); err != nil {
		httpx.Err(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, map[string]string{"status": "deleted"})
}
