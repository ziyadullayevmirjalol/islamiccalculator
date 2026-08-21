// Package service holds the application workflows: validate → load
// reference data → run the domain calculation → persist history. It
// defines the persistence interfaces; internal/repository/postgres
// implements them, tests use fakes.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/zakat"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
	"github.com/diyorbek/islamiccalculator/internal/pkg/authctx"
)

// ErrNotFound is returned by repositories when a row does not exist.
var ErrNotFound = errors.New("not found")

type MetalPrice struct {
	Metal        string
	PricePerGram decimal.Decimal
	Currency     string
	Source       string
	FetchedAt    time.Time
	Stale        bool // fetched_at older than the configured threshold
}

// markStale flags a price older than staleAfter, so clients can warn the
// user that a nisab or zakat figure rests on outdated market data.
func markStale(p *MetalPrice, staleAfter time.Duration) {
	p.Stale = staleAfter > 0 && time.Since(p.FetchedAt) > staleAfter
}

type CalculationRecord struct {
	UserID   *string // nil for anonymous calculations
	CalcType string
	Inputs   any
	Result   any
}

// newRecord builds a history record, attaching the authenticated user
// from the context when present.
func newRecord(ctx context.Context, calcType string, inputs, result any) CalculationRecord {
	rec := CalculationRecord{CalcType: calcType, Inputs: inputs, Result: result}
	if uid, ok := authctx.UserID(ctx); ok {
		rec.UserID = &uid
	}
	return rec
}

type SettingsRepo interface {
	// Value returns the JSONB value stored for key in app_settings.
	Value(ctx context.Context, key string) (json.RawMessage, error)
}

type MetalPriceRepo interface {
	// Latest returns the most recently fetched price for a metal.
	Latest(ctx context.Context, metal string) (MetalPrice, error)
}

type CalculationRepo interface {
	Save(ctx context.Context, rec CalculationRecord) error
}

type LivestockRuleRepo interface {
	// BySpecies returns the rule tiers for one species, ordered by min_count.
	BySpecies(ctx context.Context, species string) ([]zakat.LivestockRule, error)
	// All returns every rule tier, ordered by species and min_count.
	All(ctx context.Context) ([]zakat.LivestockRule, error)
}

// settingDecimal reads a single decimal field out of an app_settings
// JSONB value, e.g. settingDecimal(ctx, repo, "zakat.rate", "rate").
func settingDecimal(ctx context.Context, repo SettingsRepo, key, field string) (decimal.Decimal, error) {
	raw, err := repo.Value(ctx, key)
	if err != nil {
		return decimal.Zero, apperr.Internal(fmt.Sprintf("reference setting %q unavailable", key), err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return decimal.Zero, apperr.Internal(fmt.Sprintf("reference setting %q malformed", key), err)
	}
	d, err := decimal.NewFromString(m[field])
	if err != nil {
		return decimal.Zero, apperr.Internal(fmt.Sprintf("reference setting %q field %q malformed", key, field), err)
	}
	return d, nil
}

// settingInt reads a single integer field out of an app_settings value,
// e.g. settingInt(ctx, repo, "kaffarah.days", "days").
func settingInt(ctx context.Context, repo SettingsRepo, key, field string) (int, error) {
	raw, err := repo.Value(ctx, key)
	if err != nil {
		return 0, apperr.Internal(fmt.Sprintf("reference setting %q unavailable", key), err)
	}
	var m map[string]int
	if err := json.Unmarshal(raw, &m); err != nil {
		return 0, apperr.Internal(fmt.Sprintf("reference setting %q malformed", key), err)
	}
	n, ok := m[field]
	if !ok {
		return 0, apperr.Internal(fmt.Sprintf("reference setting %q field %q missing", key, field), nil)
	}
	return n, nil
}

// settingJSON unmarshals a whole app_settings value into dst.
func settingJSON(ctx context.Context, repo SettingsRepo, key string, dst any) error {
	raw, err := repo.Value(ctx, key)
	if err != nil {
		return apperr.Internal(fmt.Sprintf("reference setting %q unavailable", key), err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return apperr.Internal(fmt.Sprintf("reference setting %q malformed", key), err)
	}
	return nil
}
