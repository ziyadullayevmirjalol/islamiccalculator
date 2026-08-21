BEGIN;

-- AAOIFI Shariah Standard 21 financial screening thresholds. A ratio at
-- or above its threshold fails. Data, not code: a standards update is a
-- row update.
CREATE TABLE screener_rules (
    key         TEXT PRIMARY KEY,
    threshold   NUMERIC(6,4) NOT NULL CHECK (threshold > 0 AND threshold < 1),
    description TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO screener_rules (key, threshold, description) VALUES
    ('debt_to_market_cap',                 0.30, 'Interest-bearing debt / market capitalization must stay below 30%'),
    ('interest_investments_to_market_cap', 0.30, 'Cash + interest-bearing securities / market capitalization must stay below 30%'),
    ('impure_income_to_revenue',           0.05, 'Non-compliant income / total revenue must stay below 5%');

COMMIT;
