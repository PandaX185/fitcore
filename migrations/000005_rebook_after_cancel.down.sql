-- 000005_rebook_after_cancel.down.sql
-- Restore the unconditional (class_id, member_id) uniqueness.

DROP INDEX IF EXISTS uq_class_bookings_active_class_member;

ALTER TABLE class_bookings
    ADD CONSTRAINT uq_class_bookings_class_member UNIQUE (class_id, member_id);