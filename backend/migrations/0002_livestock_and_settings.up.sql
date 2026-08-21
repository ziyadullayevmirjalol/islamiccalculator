BEGIN;

-- Hanafi livestock zakat tiers as data. "due" holds the animals owed for
-- the tier; per_extra_every adds one more due-animal per that many head
-- above min_count (open-ended tiers). Cattle at 90+ head are computed in
-- code by the per-30/per-40 combination rule (see domain/zakat).
CREATE TABLE livestock_zakat_rules (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    species        TEXT  NOT NULL CHECK (species IN ('sheep_goats', 'cattle', 'camels')),
    min_count      INT   NOT NULL CHECK (min_count > 0),
    max_count      INT,
    due            JSONB NOT NULL,
    per_extra_every INT,
    note           TEXT
);

CREATE INDEX idx_livestock_rules_species ON livestock_zakat_rules (species, min_count);

INSERT INTO livestock_zakat_rules (species, min_count, max_count, due, per_extra_every, note) VALUES
    -- sheep & goats
    ('sheep_goats',  40, 120, '[{"animal": "sheep", "count": 1}]', NULL, NULL),
    ('sheep_goats', 121, 200, '[{"animal": "sheep", "count": 2}]', NULL, NULL),
    ('sheep_goats', 201, 399, '[{"animal": "sheep", "count": 3}]', NULL, NULL),
    ('sheep_goats', 400, NULL,'[{"animal": "sheep", "count": 4}]', 100,  'one_more_sheep_per_100_above_400'),
    -- cattle (incl. buffalo); 90+ computed by combination rule in code
    ('cattle', 30, 39, '[{"animal": "tabi", "count": 1}]',    NULL, NULL),
    ('cattle', 40, 59, '[{"animal": "musinna", "count": 1}]', NULL, NULL),
    ('cattle', 60, 69, '[{"animal": "tabi", "count": 2}]',    NULL, NULL),
    ('cattle', 70, 79, '[{"animal": "tabi", "count": 1}, {"animal": "musinna", "count": 1}]', NULL, NULL),
    ('cattle', 80, 89, '[{"animal": "musinna", "count": 2}]', NULL, NULL),
    ('cattle', 90, NULL, '[]', NULL, 'computed_by_combination_rule'),
    -- camels
    ('camels',   5,   9, '[{"animal": "sheep", "count": 1}]', NULL, NULL),
    ('camels',  10,  14, '[{"animal": "sheep", "count": 2}]', NULL, NULL),
    ('camels',  15,  19, '[{"animal": "sheep", "count": 3}]', NULL, NULL),
    ('camels',  20,  24, '[{"animal": "sheep", "count": 4}]', NULL, NULL),
    ('camels',  25,  35, '[{"animal": "bint_makhad", "count": 1}]', NULL, NULL),
    ('camels',  36,  45, '[{"animal": "bint_labun", "count": 1}]',  NULL, NULL),
    ('camels',  46,  60, '[{"animal": "hiqqa", "count": 1}]',       NULL, NULL),
    ('camels',  61,  75, '[{"animal": "jadhaa", "count": 1}]',      NULL, NULL),
    ('camels',  76,  90, '[{"animal": "bint_labun", "count": 2}]',  NULL, NULL),
    ('camels',  91, 120, '[{"animal": "hiqqa", "count": 2}]',       NULL, NULL),
    ('camels', 121, NULL,'[{"animal": "hiqqa", "count": 2}]',       NULL, 'above_120_rules_vary_consult_scholar');

-- Oath kaffarah feeds ten poor per broken oath (fast kaffarah of 60 is
-- already seeded as kaffarah.days).
INSERT INTO app_settings (key, value) VALUES
    ('kaffarah.oath_feedings', '{"count": 10}');

COMMIT;
