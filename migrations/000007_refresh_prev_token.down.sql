-- 000007_refresh_prev_token.down.sql
DROP INDEX IF EXISTS idx_refresh_tokens_prev_hash;

ALTER TABLE refresh_tokens
    DROP COLUMN IF EXISTS prev_expires_at,
    DROP COLUMN IF EXISTS prev_token_hash;
