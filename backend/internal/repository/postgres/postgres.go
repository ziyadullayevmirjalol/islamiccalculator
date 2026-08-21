// Package postgres implements the service-layer repository interfaces
// over pgx. NUMERIC columns are selected as text and parsed into
// decimals so no float ever touches a monetary value.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/zakat"
	"github.com/diyorbek/islamiccalculator/internal/service"
)

type Settings struct {
	pool *pgxpool.Pool
}

func NewSettings(pool *pgxpool.Pool) *Settings { return &Settings{pool: pool} }

func (r *Settings) Value(ctx context.Context, key string) (json.RawMessage, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT value FROM app_settings WHERE key = $1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("setting %q: %w", key, service.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return raw, nil
}

type Metals struct {
	pool *pgxpool.Pool
}

func NewMetals(pool *pgxpool.Pool) *Metals { return &Metals{pool: pool} }

func (r *Metals) Latest(ctx context.Context, metal string) (service.MetalPrice, error) {
	var (
		p        service.MetalPrice
		priceStr string
	)
	err := r.pool.QueryRow(ctx,
		`SELECT metal, price_per_gram::text, currency, source, fetched_at
		   FROM metal_prices
		  WHERE metal = $1
		  ORDER BY fetched_at DESC
		  LIMIT 1`, metal).
		Scan(&p.Metal, &priceStr, &p.Currency, &p.Source, &p.FetchedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.MetalPrice{}, fmt.Errorf("metal price %q: %w", metal, service.ErrNotFound)
	}
	if err != nil {
		return service.MetalPrice{}, err
	}
	if p.PricePerGram, err = decimal.NewFromString(priceStr); err != nil {
		return service.MetalPrice{}, fmt.Errorf("parse price %q: %w", priceStr, err)
	}
	return p, nil
}

func (r *Metals) Insert(ctx context.Context, p service.MetalPrice) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO metal_prices (metal, price_per_gram, currency, source, fetched_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		p.Metal, p.PricePerGram.String(), p.Currency, p.Source, p.FetchedAt)
	return err
}

type LivestockRules struct {
	pool *pgxpool.Pool
}

func NewLivestockRules(pool *pgxpool.Pool) *LivestockRules { return &LivestockRules{pool: pool} }

const livestockRuleColumns = `species, min_count, max_count, due, per_extra_every, COALESCE(note, '')`

func (r *LivestockRules) BySpecies(ctx context.Context, species string) ([]zakat.LivestockRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+livestockRuleColumns+`
		   FROM livestock_zakat_rules
		  WHERE species = $1
		  ORDER BY min_count`, species)
	if err != nil {
		return nil, err
	}
	return scanLivestockRules(rows)
}

func (r *LivestockRules) All(ctx context.Context) ([]zakat.LivestockRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+livestockRuleColumns+`
		   FROM livestock_zakat_rules
		  ORDER BY species, min_count`)
	if err != nil {
		return nil, err
	}
	return scanLivestockRules(rows)
}

func scanLivestockRules(rows pgx.Rows) ([]zakat.LivestockRule, error) {
	defer rows.Close()
	var rules []zakat.LivestockRule
	for rows.Next() {
		var (
			rule zakat.LivestockRule
			due  []byte
		)
		if err := rows.Scan(&rule.Species, &rule.MinCount, &rule.MaxCount, &due, &rule.PerExtraEvery, &rule.Note); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(due, &rule.Due); err != nil {
			return nil, fmt.Errorf("parse livestock due: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

type ScreenerRules struct {
	pool *pgxpool.Pool
}

func NewScreenerRules(pool *pgxpool.Pool) *ScreenerRules { return &ScreenerRules{pool: pool} }

func (r *ScreenerRules) All(ctx context.Context) ([]service.ScreenerRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT key, threshold::text, description FROM screener_rules ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []service.ScreenerRule
	for rows.Next() {
		var (
			rule         service.ScreenerRule
			thresholdStr string
		)
		if err := rows.Scan(&rule.Key, &thresholdStr, &rule.Description); err != nil {
			return nil, err
		}
		if rule.Threshold, err = decimal.NewFromString(thresholdStr); err != nil {
			return nil, fmt.Errorf("parse threshold %q: %w", thresholdStr, err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

type Calculations struct {
	pool *pgxpool.Pool
}

func NewCalculations(pool *pgxpool.Pool) *Calculations { return &Calculations{pool: pool} }

func (r *Calculations) Save(ctx context.Context, rec service.CalculationRecord) error {
	inputs, err := json.Marshal(rec.Inputs)
	if err != nil {
		return fmt.Errorf("marshal inputs: %w", err)
	}
	result, err := json.Marshal(rec.Result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO calculations (user_id, calc_type, inputs, result) VALUES ($1::uuid, $2, $3, $4)`,
		rec.UserID, rec.CalcType, inputs, result)
	return err
}

func (r *Calculations) ListByUser(ctx context.Context, userID string, limit int, calcType string) ([]service.HistoryEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id::text, calc_type, inputs, result, created_at
		   FROM calculations
		  WHERE user_id = $1::uuid AND ($2 = '' OR calc_type = $2)
		  ORDER BY created_at DESC
		  LIMIT $3`, userID, calcType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []service.HistoryEntry
	for rows.Next() {
		var e service.HistoryEntry
		if err := rows.Scan(&e.ID, &e.CalcType, &e.Inputs, &e.Result, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *Calculations) DeleteByUser(ctx context.Context, id, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM calculations WHERE id = $1::uuid AND user_id = $2::uuid`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return service.ErrNotFound
	}
	return nil
}

type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users { return &Users{pool: pool} }

func (r *Users) Create(ctx context.Context, email, passwordHash string) (service.User, error) {
	var u service.User
	err := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, $2)
		 RETURNING id::text, email, password_hash, created_at`,
		email, passwordHash).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return service.User{}, service.ErrEmailTaken
	}
	if err != nil {
		return service.User{}, err
	}
	return u, nil
}

func (r *Users) ByEmail(ctx context.Context, email string) (service.User, error) {
	var u service.User
	err := r.pool.QueryRow(ctx,
		`SELECT id::text, email, password_hash, created_at FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.User{}, service.ErrNotFound
	}
	if err != nil {
		return service.User{}, err
	}
	return u, nil
}

type RefreshTokens struct {
	pool *pgxpool.Pool
}

func NewRefreshTokens(pool *pgxpool.Pool) *RefreshTokens { return &RefreshTokens{pool: pool} }

func (r *RefreshTokens) Store(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1::uuid, $2, $3)`,
		userID, tokenHash, expiresAt)
	return err
}

func (r *RefreshTokens) Valid(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := r.pool.QueryRow(ctx,
		`SELECT user_id::text FROM refresh_tokens
		  WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`, tokenHash).
		Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", service.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return userID, nil
}

func (r *RefreshTokens) Revoke(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1`, tokenHash)
	return err
}
