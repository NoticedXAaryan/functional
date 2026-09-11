-- Migration 005: Cases, case access grants, and contact preferences
-- Cases are the core durable record of a support request.
-- The idempotency_key prevents duplicate cases from network retries.
-- Org-scope: every case belongs to one organization. Cross-org access is denied at the API level.

CREATE TYPE case_status AS ENUM (
    'RECEIVED',
    'TRIAGE',
    'ASSIGNED',
    'ACCEPTED',
    'IN_PROGRESS',
    'FOLLOW_UP',
    'CLOSED'
);

CREATE TABLE cases (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id     UUID NOT NULL REFERENCES organizations(id),
    entry_point_id      UUID REFERENCES entry_points(id),
    -- idempotency_key: client-generated UUID sent in Idempotency-Key header.
    -- Unique constraint prevents duplicate cases from retries.
    idempotency_key     UUID NOT NULL UNIQUE,
    status              case_status NOT NULL DEFAULT 'RECEIVED',
    route_type          VARCHAR(50) NOT NULL CHECK (route_type IN ('ask_for_help', 'worried_about_someone', 'missing_child')),
    -- account_text: the reporter's account. Access is restricted to authorized staff.
    account_text        TEXT NOT NULL,
    conflict_flag       VARCHAR(50) CHECK (conflict_flag IN ('school_implicated', 'caregiver_implicated', 'staff_implicated')),
    -- receipt_id is shown to the reporter as proof of durable receipt.
    receipt_id          UUID NOT NULL DEFAULT uuid_generate_v4() UNIQUE,
    -- version is used for optimistic locking (assignment acceptance, state transitions).
    version             INTEGER NOT NULL DEFAULT 1,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at           TIMESTAMPTZ,
    close_reason        TEXT
);

COMMENT ON TABLE cases IS
    'Durable support cases. Acknowledged only after database commit. Idempotency enforced at DB level. Cross-org access denied at API layer.';
COMMENT ON COLUMN cases.idempotency_key IS
    'Client-generated UUID. Unique constraint prevents network-retry duplicates. Changing payload under same key returns 409.';

CREATE INDEX cases_organization_id_status_idx ON cases(organization_id, status);
CREATE INDEX cases_entry_point_id_idx ON cases(entry_point_id);
CREATE INDEX cases_receipt_id_idx ON cases(receipt_id);

-- Add case_id FK to private_sessions now that cases table exists
ALTER TABLE private_sessions
    ADD CONSTRAINT private_sessions_case_id_fk
    FOREIGN KEY (case_id) REFERENCES cases(id);

-- Contact preferences are separate from case narrative (different permission scope)
CREATE TABLE contact_preferences (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    case_id             UUID NOT NULL REFERENCES cases(id) UNIQUE,
    safe_hours_start    TIME,
    safe_hours_end      TIME,
    preferred_channel   VARCHAR(50) CHECK (preferred_channel IN ('message_in_app', 'no_contact')),
    optional_contact    TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE contact_preferences IS
    'Safe contact preferences. Separate permission scope from case narrative. Staff must check this before any outreach.';

-- Case access grants: explicit grants for staff to access a specific case.
-- Organization membership alone does NOT grant case access.
CREATE TABLE case_access_grants (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    case_id         UUID NOT NULL REFERENCES cases(id),
    staff_id        UUID NOT NULL REFERENCES staff_members(id),
    -- grant_reason: assignment, supervision, escalation, audit-review
    grant_reason    VARCHAR(50) NOT NULL,
    granted_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    granted_by      UUID REFERENCES staff_members(id),
    revoked_at      TIMESTAMPTZ,
    UNIQUE (case_id, staff_id)
);

COMMENT ON TABLE case_access_grants IS
    'Explicit grants for staff to access a specific case. Org membership alone is insufficient.';

CREATE INDEX case_access_grants_case_id_staff_idx ON case_access_grants(case_id, staff_id);

-- State transition event log — every state change is audited
CREATE TABLE case_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    case_id         UUID NOT NULL REFERENCES cases(id),
    from_status     case_status,
    to_status       case_status NOT NULL,
    actor_id        UUID REFERENCES staff_members(id),
    actor_type      VARCHAR(20) NOT NULL CHECK (actor_type IN ('system', 'staff', 'session')),
    reason          TEXT,
    case_version    INTEGER NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX case_events_case_id_idx ON case_events(case_id);
