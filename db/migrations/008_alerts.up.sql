-- Migration 008: Alerts, revisions, approvals, and outbox
-- Alert content moves through: Draft → In_Review → Approved → Active → Resolved/Withdrawn/Expired
-- EVERY content change creates a NEW revision — existing approved content is immutable.
-- Approval binds the complete revision; any field change invalidates it.

CREATE TYPE alert_status AS ENUM (
    'DRAFT',
    'IN_REVIEW',
    'APPROVED',
    'ACTIVE',
    'RESOLVED',
    'WITHDRAWN',
    'EXPIRED'
);

CREATE TABLE alerts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- case_id is an internal link only. Never exposed through the public API.
    case_id         UUID NOT NULL REFERENCES cases(id),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    status          alert_status NOT NULL DEFAULT 'DRAFT',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- active_revision_id is set only when status=ACTIVE.
    active_revision_id UUID
);

COMMENT ON TABLE alerts IS
    'Alert lifecycle container. Status drives the approval and delivery flow. The public API never exposes case_id or private case data.';

-- An AlertRevision holds the exact content that was reviewed and approved.
-- Once approved, the revision is immutable.
CREATE TABLE alert_revisions (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    alert_id            UUID NOT NULL REFERENCES alerts(id),
    revision_number     INTEGER NOT NULL,
    status              alert_status NOT NULL DEFAULT 'DRAFT',
    -- Public content fields — approved projection only
    description_text    VARCHAR(500) NOT NULL,
    issuer_name         VARCHAR(255) NOT NULL,
    -- coverage_polygon is a GeoJSON Polygon (stored as PostGIS geometry)
    coverage_polygon    geometry(Polygon, 4326),
    area_version        VARCHAR(50),
    last_seen_area      VARCHAR(255),
    expiry_at           TIMESTAMPTZ NOT NULL,
    tip_route_email     VARCHAR(255) NOT NULL,
    prepared_by         UUID NOT NULL REFERENCES staff_members(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (alert_id, revision_number)
);

COMMENT ON TABLE alert_revisions IS
    'Immutable content snapshots. Approval binds the full revision. Any field change creates a new revision and invalidates activation against the old one.';

CREATE INDEX alert_revisions_alert_id_idx ON alert_revisions(alert_id);

-- Approval records: preparer ≠ approver (enforced at API level)
CREATE TABLE alert_approvals (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    alert_id        UUID NOT NULL REFERENCES alerts(id),
    revision_id     UUID NOT NULL REFERENCES alert_revisions(id) UNIQUE,
    approver_id     UUID NOT NULL REFERENCES staff_members(id),
    decision        VARCHAR(10) NOT NULL CHECK (decision IN ('approve', 'reject')),
    notes           TEXT,
    decided_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE alert_approvals IS
    'One approval record per revision. UNIQUE(revision_id) enforces immutability. Self-approval is rejected at the API layer.';

-- Outbox: transactional outbox pattern for reliable delivery.
-- Written in the same transaction as alert activation.
CREATE TABLE outbox_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type      VARCHAR(50) NOT NULL,
    alert_id        UUID REFERENCES alerts(id),
    revision_id     UUID REFERENCES alert_revisions(id),
    payload         JSONB NOT NULL DEFAULT '{}',
    status          VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSING', 'DONE', 'FAILED')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at    TIMESTAMPTZ,
    error_message   TEXT
);

CREATE INDEX outbox_events_status_idx ON outbox_events(status, created_at)
    WHERE status IN ('PENDING', 'PROCESSING');

-- Add the FK from alerts.active_revision_id now that alert_revisions exists
ALTER TABLE alerts
    ADD CONSTRAINT alerts_active_revision_id_fk
    FOREIGN KEY (active_revision_id) REFERENCES alert_revisions(id);
