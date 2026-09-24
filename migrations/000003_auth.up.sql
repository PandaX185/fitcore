-- 000003_auth.up.sql
-- Staff authentication: credentials, permission sets and rotating refresh
-- tokens. The role column is replaced by explicit permission sets (stored as
-- a JSON array so the pgx/GORM stack scans it cleanly); each staff member
-- carries the grants they hold, re-read at login and on every refresh.

ALTER TABLE staff
    DROP COLUMN role;

ALTER TABLE staff
    ADD COLUMN password_hash text,
    ADD COLUMN permissions jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN active boolean NOT NULL DEFAULT true;

CREATE TABLE refresh_tokens (
    id          uuid        NOT NULL DEFAULT uuid_generate_v4(),
    staff_id    uuid        NOT NULL,
    token_hash  text        NOT NULL,
    jti         uuid        NOT NULL,
    expires_at  timestamptz NOT NULL,
    revoked     boolean     NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_refresh_tokens PRIMARY KEY (id),
    CONSTRAINT uq_refresh_tokens_token_hash UNIQUE (token_hash),
    CONSTRAINT fk_refresh_tokens_staff FOREIGN KEY (staff_id)
        REFERENCES staff (id) ON DELETE CASCADE
);
CREATE INDEX idx_refresh_tokens_staff ON refresh_tokens (staff_id);
CREATE INDEX idx_refresh_tokens_jti ON refresh_tokens (jti);