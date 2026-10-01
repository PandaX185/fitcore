-- 000005_rebook_after_cancel.down.sql
-- UNSAFE DOWN MIGRATION WITH REAL DATA: restoring the unconditional
-- (class_id, member_id) uniqueness fails when any member rebooked a class
-- after cancelling (two rows share one class_id + member_id pair). Do not run
-- this down migration on a database that contains such rebooked rows; delete
-- or consolidate the cancelled history rows first. The guard below aborts
-- cleanly instead of half-applying (dropping the partial index and then
-- failing to re-add the constraint).

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM class_bookings
        GROUP BY class_id, member_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'Down migration 000005 blocked: class_bookings contains rebooked-after-cancel rows (duplicate class_id + member_id pairs). Remove or consolidate cancelled history rows before downgrading.';
    END IF;
END
$$;

DROP INDEX IF EXISTS uq_class_bookings_active_class_member;

ALTER TABLE class_bookings
    ADD CONSTRAINT uq_class_bookings_class_member UNIQUE (class_id, member_id);
