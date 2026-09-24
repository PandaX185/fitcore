-- 000005_rebook_after_cancel.up.sql
-- Let a member book a class seat again after cancelling their previous
-- booking: the (class_id, member_id) uniqueness must apply only to live
-- (booked) bookings, not to cancelled history rows.

ALTER TABLE class_bookings
    DROP CONSTRAINT uq_class_bookings_class_member;

CREATE UNIQUE INDEX uq_class_bookings_active_class_member
    ON class_bookings (class_id, member_id)
    WHERE status = 'booked';