-- 000007_refresh_prev_token.up.sql
-- Retain the immediate predecessor of each rotating refresh token so a
-- benign client retry (lost rotation response) can be told apart from token
-- theft: rotation copies the current hash/expiry into prev_token_hash /
-- prev_expires_at before overwriting, and replays of the predecessor inside
-- the grace window are rejected without killing the family.

ALTER TABLE refresh_tokens
    ADD COLUMN prev_token_hash text,
    ADD COLUMN prev_expires_at timestamptz;

CREATE INDEX idx_refresh_tokens_prev_hash ON refresh_tokens (prev_token_hash);
