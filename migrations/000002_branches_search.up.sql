-- 000002_branches_search.up.sql
-- Accelerate case-insensitive branch name search with trigram (GIN) indexing.
-- A PostGIS/GiST index for proximity search is intentionally deferred until a
-- "find gyms near me" requirement exists; lat/lon are currently data, not
-- search keys.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_branches_name_trgm ON branches USING GIN (name gin_trgm_ops);