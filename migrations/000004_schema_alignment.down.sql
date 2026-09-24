-- 000004_schema_alignment.down.sql
-- Reverse the schema alignment: strip the contract columns and restore the
-- original date-based columns.

ALTER TABLE trainers
    DROP COLUMN active;

ALTER TABLE invoices
    ALTER COLUMN issued_on DROP DEFAULT,
    ALTER COLUMN issued_on TYPE date USING (issued_on::date),
    ALTER COLUMN issued_on SET DEFAULT CURRENT_DATE;
ALTER TABLE invoices
    ALTER COLUMN paid_on TYPE date USING (paid_on::date);

ALTER TABLE invoices
    DROP CONSTRAINT IF EXISTS chk_invoices_currency,
    DROP CONSTRAINT IF EXISTS fk_invoices_membership,
    DROP COLUMN IF EXISTS due_at,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS membership_id;

DROP INDEX IF EXISTS idx_attendance_branch_checked_in;

ALTER TABLE attendance
    DROP CONSTRAINT IF EXISTS fk_attendance_membership,
    DROP CONSTRAINT IF EXISTS fk_attendance_branch,
    DROP COLUMN IF EXISTS checked_out_at,
    DROP COLUMN IF EXISTS membership_id,
    DROP COLUMN IF EXISTS branch_id;

ALTER TABLE class_bookings
    DROP COLUMN IF EXISTS cancelled_at;

ALTER TABLE memberships
    ALTER COLUMN starts_on TYPE date USING (starts_on::date);
ALTER TABLE memberships
    ALTER COLUMN expires_on TYPE date USING (expires_on::date);

ALTER TABLE memberships
    DROP CONSTRAINT IF EXISTS fk_memberships_branch,
    DROP COLUMN IF EXISTS branch_id;

ALTER TABLE membership_packages
    DROP CONSTRAINT IF EXISTS chk_membership_packages_currency,
    DROP COLUMN IF EXISTS currency;