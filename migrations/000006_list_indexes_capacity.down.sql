-- 000006_list_indexes_capacity.down.sql
-- Reverse 000006 in opposite order: drop the trigram index, restore the
-- original capacity check, drop the unique indexes, then the FK and
-- list-ordering indexes.

DROP INDEX IF EXISTS idx_branches_address_trgm;

ALTER TABLE classes DROP CONSTRAINT IF EXISTS chk_classes_capacity;
ALTER TABLE classes ADD CONSTRAINT chk_classes_capacity CHECK (capacity >= 0);

DROP INDEX IF EXISTS uq_invoices_terms;
DROP INDEX IF EXISTS uq_attendance_open_member;

DROP INDEX IF EXISTS idx_invoices_membership;
DROP INDEX IF EXISTS idx_attendance_branch;
DROP INDEX IF EXISTS idx_attendance_class;
DROP INDEX IF EXISTS idx_attendance_membership;
DROP INDEX IF EXISTS idx_memberships_branch;
DROP INDEX IF EXISTS idx_memberships_package;
DROP INDEX IF EXISTS idx_memberships_member;
DROP INDEX IF EXISTS idx_members_branch;
DROP INDEX IF EXISTS idx_class_bookings_member;

DROP INDEX IF EXISTS idx_attendance_open_member_checked_in;
DROP INDEX IF EXISTS idx_invoices_member_issued_id;
DROP INDEX IF EXISTS idx_attendance_member_checked_in_id;
DROP INDEX IF EXISTS idx_memberships_member_starts_id;
DROP INDEX IF EXISTS idx_class_bookings_class_created_id;
DROP INDEX IF EXISTS idx_classes_trainer_starts_id;
DROP INDEX IF EXISTS idx_classes_branch_starts_id;
DROP INDEX IF EXISTS idx_trainers_branch_name_id;
DROP INDEX IF EXISTS idx_staff_branch_name_id;
DROP INDEX IF EXISTS idx_branches_name_id;
DROP INDEX IF EXISTS idx_membership_packages_name_id;
DROP INDEX IF EXISTS idx_members_name_id;
