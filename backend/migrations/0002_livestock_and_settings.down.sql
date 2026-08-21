BEGIN;

DELETE FROM app_settings WHERE key = 'kaffarah.oath_feedings';
DROP TABLE IF EXISTS livestock_zakat_rules;

COMMIT;
