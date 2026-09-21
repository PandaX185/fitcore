-- 000001_init.down.sql
-- Rollback of the initial FitCore schema, reverse dependency order.

DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS attendance;
DROP TABLE IF EXISTS class_bookings;
DROP TABLE IF EXISTS classes;
DROP TABLE IF EXISTS memberships;
DROP TABLE IF EXISTS membership_packages;
DROP TABLE IF EXISTS members;
DROP TABLE IF EXISTS trainers;
DROP TABLE IF EXISTS staff;
DROP TABLE IF EXISTS branches;

DROP EXTENSION IF EXISTS "uuid-ossp";
