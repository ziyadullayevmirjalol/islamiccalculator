package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

type HistoryEntry struct {
	ID        string
	CalcType  string
	Inputs    json.RawMessage
	Result    json.RawMessage
	CreatedAt time.Time
}

type HistoryRepo interface {
	// ListByUser returns the user's newest entries, optionally filtered
	// by calc type.
	ListByUser(ctx context.Context, userID string, limit int, calcType string) ([]HistoryEntry, error)
	// DeleteByUser removes one entry owned by the user; ErrNotFound when
	// the id doesn't exist or belongs to someone else.
	DeleteByUser(ctx context.Context, id, userID string) error
}

type History struct {
	repo HistoryRepo
}

func NewHistory(repo HistoryRepo) *History {
	return &History{repo: repo}
}

func (s *History) List(ctx context.Context, userID string, limit int, calcType string) ([]HistoryEntry, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	entries, err := s.repo.ListByUser(ctx, userID, limit, calcType)
	if err != nil {
		return nil, apperr.Internal("load history", err)
	}
	return entries, nil
}

func (s *History) Delete(ctx context.Context, userID, id string) error {
	err := s.repo.DeleteByUser(ctx, id, userID)
	if errors.Is(err, ErrNotFound) {
		return apperr.NotFound("calculation not found")
	}
	if err != nil {
		return apperr.Internal("delete history entry", err)
	}
	return nil
}
