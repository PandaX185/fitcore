-- 000006_list_indexes_capacity.up.sql
-- Cover every list ordering with a btree composite index, add FK lookup
-- indexes, enforce a single open attendance visit per member, disambiguate
-- invoice terms, tighten class capacity to > 0, and accelerate the
-- branch address trigram search (pg_trgm pattern follows 000002).

-- List-ordering composites.
CREATE INDEX IF NOT EXISTS idx_members_name_id ON members (name, id);
CREATE INDEX IF NOT EXISTS idx_membership_packages_name_id ON membership_packages (name, id);
CREATE INDEX IF NOT EXISTS idx_branches_name_id ON branches (name, id);
CREATE INDEX IF NOT EXISTS idx_staff_branch_name_id ON staff (branch_id, name, id);
CREATE INDEX IF NOT EXISTS idx_trainers_branch_name_id ON trainers (branch_id, name, id);
CREATE INDEX IF NOT EXISTS idx_classes_branch_starts_id ON classes (branch_id, starts_at, id);
CREATE INDEX IF NOT EXISTS idx_classes_trainer_starts_id ON classes (trainer_id, starts_at, id);
CREATE INDEX IF NOT EXISTS idx_class_bookings_class_created_id ON class_bookings (class_id, created_at, id);
CREATE INDEX IF NOT EXISTS idx_memberships_member_starts_id ON memberships (member_id, starts_on DESC, id);
CREATE INDEX IF NOT EXISTS idx_attendance_member_checked_in_id ON attendance (member_id, checked_in_at DESC, id);
CREATE INDEX IF NOT EXISTS idx_invoices_member_issued_id ON invoices (member_id, issued_on DESC, id);

-- Partial index for the open-visit lookup (member_id + checked_out_at IS NULL,
-- ordered by most recent check-in).
CREATE INDEX IF NOT EXISTS idx_attendance_open_member_checked_in
    ON attendance (member_id, checked_in_at DESC, id)
    WHERE checked_out_at IS NULL;

-- FK lookup indexes (staff/trainers/classes branch + trainer legs are covered
-- by the composites above).
CREATE INDEX IF NOT EXISTS idx_class_bookings_member ON class_bookings (member_id);
CREATE INDEX IF NOT EXISTS idx_members_branch ON members (branch_id);
CREATE INDEX IF NOT EXISTS idx_memberships_member ON memberships (member_id);
CREATE INDEX IF NOT EXISTS idx_memberships_package ON memberships (package_id);
CREATE INDEX IF NOT EXISTS idx_memberships_branch ON memberships (branch_id);
CREATE INDEX IF NOT EXISTS idx_attendance_membership ON attendance (membership_id);
CREATE INDEX IF NOT EXISTS idx_attendance_class ON attendance (class_id);
CREATE INDEX IF NOT EXISTS idx_attendance_branch ON attendance (branch_id);
CREATE INDEX IF NOT EXISTS idx_invoices_membership ON invoices (membership_id);

-- One open visit per member.
CREATE UNIQUE INDEX IF NOT EXISTS uq_attendance_open_member
    ON attendance (member_id)
    WHERE checked_out_at IS NULL;

-- Disambiguate invoice terms per membership.
CREATE UNIQUE INDEX IF NOT EXISTS uq_invoices_terms
    ON invoices (membership_id, amount_cents, due_at);

-- Classes are only creatable with capacity > 0 at the service layer, so no
-- zero-capacity rows can exist; tighten the check (was capacity >= 0).
ALTER TABLE classes DROP CONSTRAINT IF EXISTS chk_classes_capacity;
ALTER TABLE classes ADD CONSTRAINT chk_classes_capacity CHECK (capacity > 0);

-- Trigram search on branch address (name already covered by 000002).
CREATE INDEX IF NOT EXISTS idx_branches_address_trgm ON branches USING GIN (address gin_trgm_ops);
