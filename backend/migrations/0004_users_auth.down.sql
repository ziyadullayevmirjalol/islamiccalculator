BEGIN;

ALTER TABLE calculations DROP CONSTRAINT IF EXISTS fk_calculations_user;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;

COMMIT;
