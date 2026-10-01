-- 000003_auth.down.sql
-- Restore the original pre-auth schema from 000001 (role had no DEFAULT).
DROP TABLE IF EXISTS refresh_tokens;

ALTER TABLE staff
    DROP COLUMN IF EXISTS permissions,
    DROP COLUMN IF EXISTS password_hash,
    DROP COLUMN IF EXISTS active,
    ADD COLUMN role text NOT NULL;