-- 000002_branches_search.up.sql
-- Accelerate case-insensitive branch name search with trigram (GIN) indexing.
-- Proximity search ("find gyms near me") has no index yet and is intentionally
-- deferred until that requirement exists; lat/lon are currently data, not
-- search keys.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_branches_name_trgm ON branches USING GIN (name gin_trgm_ops);