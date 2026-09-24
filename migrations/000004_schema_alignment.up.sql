-- 000004_schema_alignment.up.sql
-- Align the schema with the OpenAPI contract for the remaining modules:
-- package currency, memberships tied to a branch with timestamp bounds,
-- booking cancellation, attendance branch/membership/check-out columns,
-- invoice membership/due-date/currency columns, and trainer activation.

ALTER TABLE membership_packages
    ADD COLUMN currency text NOT NULL DEFAULT 'BHD',
    ADD CONSTRAINT chk_membership_packages_currency CHECK (char_length(currency) = 3);

ALTER TABLE memberships
    ADD COLUMN branch_id uuid,
    ADD CONSTRAINT fk_memberships_branch FOREIGN KEY (branch_id)
        REFERENCES branches (id) ON DELETE SET NULL;

ALTER TABLE memberships
    ALTER COLUMN starts_on TYPE timestamptz USING (starts_on::timestamptz);
ALTER TABLE memberships
    ALTER COLUMN expires_on TYPE timestamptz USING (expires_on::timestamptz);

ALTER TABLE class_bookings
    ADD COLUMN cancelled_at timestamptz;

ALTER TABLE attendance
    ADD COLUMN branch_id uuid,
    ADD COLUMN membership_id uuid,
    ADD COLUMN checked_out_at timestamptz,
    ADD CONSTRAINT fk_attendance_branch FOREIGN KEY (branch_id)
        REFERENCES branches (id) ON DELETE SET NULL,
    ADD CONSTRAINT fk_attendance_membership FOREIGN KEY (membership_id)
        REFERENCES memberships (id) ON DELETE SET NULL;

CREATE INDEX idx_attendance_branch_checked_in ON attendance (branch_id, checked_in_at);

ALTER TABLE invoices
    ADD COLUMN membership_id uuid,
    ADD COLUMN currency text NOT NULL DEFAULT 'BHD',
    ADD COLUMN due_at timestamptz NOT NULL DEFAULT now(),
    ADD CONSTRAINT fk_invoices_membership FOREIGN KEY (membership_id)
        REFERENCES memberships (id) ON DELETE SET NULL,
    ADD CONSTRAINT chk_invoices_currency CHECK (char_length(currency) = 3);

ALTER TABLE invoices
    ALTER COLUMN issued_on DROP DEFAULT,
    ALTER COLUMN issued_on TYPE timestamptz USING (issued_on::timestamptz),
    ALTER COLUMN issued_on SET DEFAULT now();
ALTER TABLE invoices
    ALTER COLUMN paid_on TYPE timestamptz USING (paid_on::timestamptz);

ALTER TABLE trainers
    ADD COLUMN active boolean NOT NULL DEFAULT true;