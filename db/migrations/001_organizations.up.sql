-- Migration 001: Organizations
-- Represents the service organizations (NGOs, schools' support bodies, etc.)
-- that employ responders and own QR placements.

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE organizations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(100) NOT NULL UNIQUE,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE organizations IS
    'Participating support organizations. Every case, staff member, and QR placement belongs to exactly one organization.';

-- Seed a demo organization for synthetic fixtures only.
-- All values are FICTIONAL.
INSERT INTO organizations (id, name, slug) VALUES
    ('00000000-0000-0000-0000-000000000001', '[FICTIONAL] Nagar Support Services Demo', 'nagar-support-demo');
