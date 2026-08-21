BEGIN;

-- Fiqh parameters as data, not code. Hanafi defaults; values pending
-- scholar review before launch (see PLAN.md §11).
CREATE TABLE app_settings (
    key        TEXT PRIMARY KEY,
    value      JSONB       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO app_settings (key, value) VALUES
    ('zakat.rate',              '{"rate": "0.025"}'),
    ('zakat.nisab_gold_grams',  '{"grams": "87.48"}'),
    ('zakat.nisab_silver_grams','{"grams": "612.36"}'),
    ('ushr.natural_rate',       '{"rate": "0.10"}'),
    ('ushr.irrigated_rate',     '{"rate": "0.05"}'),
    ('fidya.daily',             '{"amount": "15000", "currency": "UZS", "needs_review": true}'),
    ('kaffarah.days',           '{"days": 60}');

-- Spot price cache; the latest row per metal drives the nisab value.
CREATE TABLE metal_prices (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    metal          TEXT           NOT NULL CHECK (metal IN ('gold', 'silver')),
    price_per_gram NUMERIC(24,4)  NOT NULL CHECK (price_per_gram > 0),
    currency       CHAR(3)        NOT NULL DEFAULT 'UZS',
    source         TEXT           NOT NULL,
    fetched_at     TIMESTAMPTZ    NOT NULL DEFAULT now()
);

CREATE INDEX idx_metal_prices_latest ON metal_prices (metal, fetched_at DESC);

-- Seed prices so /rates/metals works before a live provider exists.
-- source='seed' makes staleness visible to clients and admins.
INSERT INTO metal_prices (metal, price_per_gram, currency, source) VALUES
    ('gold',   '1450000.0000', 'UZS', 'seed'),
    ('silver',   '17500.0000', 'UZS', 'seed');

-- Calculation history. Anonymous rows have user_id NULL; the users table
-- arrives in phase 6, so no FK yet.
CREATE TABLE calculations (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID,
    calc_type  TEXT        NOT NULL,
    inputs     JSONB       NOT NULL,
    result     JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_calculations_type    ON calculations (calc_type, created_at DESC);
CREATE INDEX idx_calculations_user_id ON calculations (user_id) WHERE user_id IS NOT NULL;

COMMIT;
