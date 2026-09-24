-- 000003_auth.down.sql
DROP TABLE IF EXISTS refresh_tokens;

ALTER TABLE staff
    DROP COLUMN IF EXISTS permissions,
    DROP COLUMN IF EXISTS password_hash,
    DROP COLUMN IF EXISTS active,
    ADD COLUMN role text NOT NULL DEFAULT 'front_desk';