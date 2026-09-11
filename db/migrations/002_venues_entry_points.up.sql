-- Migration 002: Venues and Entry Points
-- A Venue is a physical location (school, community center, etc.).
-- An EntryPoint is a specific QR code placement at a venue.
-- Owning an EntryPoint does NOT grant access to cases submitted through it.

CREATE TABLE venues (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name            VARCHAR(255) NOT NULL,
    address         TEXT,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE venues IS
    'Physical locations where QR placements may be installed. Venue ownership does not grant case access.';

CREATE TABLE entry_points (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    venue_id        UUID NOT NULL REFERENCES venues(id),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    -- opaque_id is the value embedded in the QR code URL.
    -- It is a random token, not a UUID, to prevent enumeration.
    opaque_id       VARCHAR(64) NOT NULL UNIQUE,
    label           VARCHAR(255),
    languages       TEXT[] NOT NULL DEFAULT ARRAY['en'],
    service_hours   VARCHAR(255),
    -- Routes available at this placement
    routes_available TEXT[] NOT NULL DEFAULT ARRAY['check_and_leave', 'ask_for_help', 'worried_about_someone', 'missing_child'],
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    revoked_at      TIMESTAMPTZ,
    revoke_reason   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE entry_points IS
    'QR placements. Each has an opaque_id embedded in the QR URL to prevent enumeration. Revocation removes access without exposing case data.';

CREATE INDEX entry_points_opaque_id_idx ON entry_points(opaque_id);
CREATE INDEX entry_points_organization_id_idx ON entry_points(organization_id);

-- FICTIONAL demo venue and entry point
INSERT INTO venues (id, organization_id, name) VALUES
    ('00000000-0000-0000-0000-000000000010', '00000000-0000-0000-0000-000000000001', '[FICTIONAL] Nagar Community Hall');

INSERT INTO entry_points (id, venue_id, organization_id, opaque_id, label) VALUES
    ('00000000-0000-0000-0000-000000000011',
     '00000000-0000-0000-0000-000000000010',
     '00000000-0000-0000-0000-000000000001',
     'demo-qr-nagar-hall-001',
     '[FICTIONAL] Main entrance QR - Demo');
