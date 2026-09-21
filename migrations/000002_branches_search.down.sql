-- 000002_branches_search.down.sql
-- Drop the search index. The pg_trgm extension is left installed because it
-- may be shared by other indexes.

DROP INDEX IF EXISTS idx_branches_name_trgm;