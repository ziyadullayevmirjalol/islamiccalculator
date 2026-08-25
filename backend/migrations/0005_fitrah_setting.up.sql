BEGIN;

-- One sa' (a volume measure) expressed in kg of staple food.
-- Scholarly estimates run 2.0–3.0 kg; 2.5 kg is the common default.
INSERT INTO app_settings (key, value) VALUES
    ('fitrah.sa_kg', '{"kg": "2.5", "needs_review": true}')
ON CONFLICT (key) DO NOTHING;

COMMIT;
