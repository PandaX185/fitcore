-- 000001_init.up.sql
-- Initial FitCore schema: physical locations, people, classes, memberships,
-- bookings, attendance and billing. All constraints are named so they can be
-- altered in later migrations.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE branches (
    id          uuid        NOT NULL DEFAULT uuid_generate_v4(),
    name        text        NOT NULL,
    address     text        NOT NULL DEFAULT '',
    latitude    double precision,
    longitude   double precision,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_branches PRIMARY KEY (id)
);

CREATE TABLE staff (
    id          uuid        NOT NULL DEFAULT uuid_generate_v4(),
    branch_id   uuid,
    name        text        NOT NULL,
    email       text        NOT NULL,
    phone       text        NOT NULL DEFAULT '',
    role        text        NOT NULL,
    hire_date   date,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_staff PRIMARY KEY (id),
    CONSTRAINT uq_staff_email UNIQUE (email),
    CONSTRAINT fk_staff_branch FOREIGN KEY (branch_id)
        REFERENCES branches (id) ON DELETE SET NULL
);

CREATE TABLE trainers (
    id          uuid        NOT NULL DEFAULT uuid_generate_v4(),
    branch_id   uuid,
    name        text        NOT NULL,
    email       text        NOT NULL,
    phone       text        NOT NULL DEFAULT '',
    specialties text[]      NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_trainers PRIMARY KEY (id),
    CONSTRAINT uq_trainers_email UNIQUE (email),
    CONSTRAINT fk_trainers_branch FOREIGN KEY (branch_id)
        REFERENCES branches (id) ON DELETE SET NULL
);

CREATE TABLE members (
    id         uuid        NOT NULL DEFAULT uuid_generate_v4(),
    branch_id  uuid,
    name       text        NOT NULL,
    email      text        NOT NULL,
    phone      text        NOT NULL DEFAULT '',
    status     text        NOT NULL DEFAULT 'active',
    joined_at  date,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_members PRIMARY KEY (id),
    CONSTRAINT uq_members_email UNIQUE (email),
    CONSTRAINT fk_members_branch FOREIGN KEY (branch_id)
        REFERENCES branches (id) ON DELETE SET NULL
);

CREATE TABLE membership_packages (
    id            uuid        NOT NULL DEFAULT uuid_generate_v4(),
    name          text        NOT NULL,
    description   text        NOT NULL DEFAULT '',
    duration_days integer     NOT NULL,
    price_cents   integer     NOT NULL,
    active        boolean     NOT NULL DEFAULT true,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_membership_packages PRIMARY KEY (id),
    CONSTRAINT uq_membership_packages_name UNIQUE (name),
    CONSTRAINT chk_membership_packages_duration_days CHECK (duration_days > 0),
    CONSTRAINT chk_membership_packages_price_cents CHECK (price_cents >= 0)
);

CREATE TABLE memberships (
    id         uuid        NOT NULL DEFAULT uuid_generate_v4(),
    member_id  uuid        NOT NULL,
    package_id uuid,
    starts_on  date        NOT NULL,
    expires_on date        NOT NULL,
    status     text        NOT NULL DEFAULT 'active',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_memberships PRIMARY KEY (id),
    CONSTRAINT fk_memberships_member FOREIGN KEY (member_id)
        REFERENCES members (id) ON DELETE CASCADE,
    CONSTRAINT fk_memberships_package FOREIGN KEY (package_id)
        REFERENCES membership_packages (id) ON DELETE SET NULL,
    CONSTRAINT chk_memberships_dates CHECK (expires_on >= starts_on)
);

-- A member cannot hold two overlapping active memberships; historical rows are
-- retired via status change.
CREATE UNIQUE INDEX uq_memberships_active_member
    ON memberships (member_id)
    WHERE status = 'active';

CREATE TABLE classes (
    id          uuid        NOT NULL DEFAULT uuid_generate_v4(),
    branch_id   uuid,
    trainer_id  uuid,
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    capacity    integer     NOT NULL DEFAULT 0,
    starts_at   timestamptz NOT NULL,
    ends_at     timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_classes PRIMARY KEY (id),
    CONSTRAINT chk_classes_capacity CHECK (capacity >= 0),
    CONSTRAINT chk_classes_time_range CHECK (ends_at > starts_at),
    CONSTRAINT fk_classes_branch FOREIGN KEY (branch_id)
        REFERENCES branches (id) ON DELETE SET NULL,
    CONSTRAINT fk_classes_trainer FOREIGN KEY (trainer_id)
        REFERENCES trainers (id) ON DELETE SET NULL
);
CREATE INDEX idx_classes_starts_at ON classes (starts_at);

CREATE TABLE class_bookings (
    id         uuid        NOT NULL DEFAULT uuid_generate_v4(),
    class_id   uuid        NOT NULL,
    member_id  uuid        NOT NULL,
    status     text        NOT NULL DEFAULT 'booked',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_class_bookings PRIMARY KEY (id),
    CONSTRAINT uq_class_bookings_class_member UNIQUE (class_id, member_id),
    CONSTRAINT fk_class_bookings_class FOREIGN KEY (class_id)
        REFERENCES classes (id) ON DELETE CASCADE,
    CONSTRAINT fk_class_bookings_member FOREIGN KEY (member_id)
        REFERENCES members (id) ON DELETE CASCADE
);
CREATE INDEX idx_class_bookings_class ON class_bookings (class_id);

CREATE TABLE attendance (
    id            uuid        NOT NULL DEFAULT uuid_generate_v4(),
    member_id     uuid        NOT NULL,
    class_id      uuid,
    checked_in_at timestamptz NOT NULL DEFAULT now(),
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_attendance PRIMARY KEY (id),
    CONSTRAINT fk_attendance_member FOREIGN KEY (member_id)
        REFERENCES members (id) ON DELETE CASCADE,
    CONSTRAINT fk_attendance_class FOREIGN KEY (class_id)
        REFERENCES classes (id) ON DELETE SET NULL
);
CREATE INDEX idx_attendance_member_checked_in ON attendance (member_id, checked_in_at);

CREATE TABLE invoices (
    id           uuid        NOT NULL DEFAULT uuid_generate_v4(),
    member_id    uuid        NOT NULL,
    amount_cents integer     NOT NULL,
    status       text        NOT NULL DEFAULT 'pending',
    description  text        NOT NULL DEFAULT '',
    issued_on    date        NOT NULL DEFAULT CURRENT_DATE,
    paid_on      date,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT pk_invoices PRIMARY KEY (id),
    CONSTRAINT chk_invoices_amount_cents CHECK (amount_cents >= 0),
    CONSTRAINT fk_invoices_member FOREIGN KEY (member_id)
        REFERENCES members (id) ON DELETE CASCADE
);
CREATE INDEX idx_invoices_member_status ON invoices (member_id, status);