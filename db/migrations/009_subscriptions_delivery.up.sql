-- Migration 009: Area subscriptions, delivery jobs, attempts, and tips

-- Area subscriptions: adults subscribe to geographic areas to receive alerts.
-- FCM token is stored here. Separate from case sessions — no linking.
CREATE TABLE area_subscriptions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- area_name is a human-readable label (e.g. "Nagar Ward 4")
    area_name       VARCHAR(255) NOT NULL,
    -- polygon is the area the subscriber has chosen (PostGIS geometry)
    polygon         geometry(Polygon, 4326),
    fcm_token       TEXT NOT NULL,
    language        VARCHAR(10) NOT NULL DEFAULT 'en',
    -- consent fields
    consent_confirmed_at TIMESTAMPTZ NOT NULL,
    consent_purpose TEXT NOT NULL DEFAULT 'Receive verified missing-child alerts for the chosen area.',
    -- is_test: TRUE means this endpoint is in the TEST allowlist
    is_test         BOOLEAN NOT NULL DEFAULT FALSE,
    subscribed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at      TIMESTAMPTZ
);

COMMENT ON TABLE area_subscriptions IS
    'Alert subscriptions. TEST endpoints are isolated in a separate allowlist (max 20). No linking to Incognito sessions.';

CREATE INDEX area_subscriptions_polygon_idx ON area_subscriptions USING GIST(polygon)
    WHERE revoked_at IS NULL;
CREATE INDEX area_subscriptions_is_test_idx ON area_subscriptions(is_test)
    WHERE revoked_at IS NULL;

-- Delivery jobs: one per subscriber/channel/revision combination.
-- Unique key prevents duplicates across retries.
CREATE TABLE delivery_jobs (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    outbox_event_id     UUID NOT NULL REFERENCES outbox_events(id),
    alert_id            UUID NOT NULL REFERENCES alerts(id),
    revision_id         UUID NOT NULL REFERENCES alert_revisions(id),
    subscription_id     UUID NOT NULL REFERENCES area_subscriptions(id),
    status              VARCHAR(20) NOT NULL DEFAULT 'QUEUED'
        CHECK (status IN ('QUEUED', 'PROCESSING', 'SENT', 'FAILED', 'CANCELLED', 'EXPIRED')),
    is_test             BOOLEAN NOT NULL DEFAULT FALSE,
    scheduled_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Unique: one logical delivery per revision+subscription
    UNIQUE (revision_id, subscription_id)
);

COMMENT ON TABLE delivery_jobs IS
    'One logical delivery job per revision+subscription. UNIQUE constraint prevents duplicates across retries. is_test must be true for all TEST_ALLOWLIST sends.';

CREATE INDEX delivery_jobs_status_idx ON delivery_jobs(status, scheduled_at)
    WHERE status IN ('QUEUED', 'PROCESSING');
CREATE INDEX delivery_jobs_alert_id_idx ON delivery_jobs(alert_id);

-- Delivery attempts: each send attempt for a job (bounded retries)
CREATE TABLE delivery_attempts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id          UUID NOT NULL REFERENCES delivery_jobs(id),
    attempt_number  INTEGER NOT NULL,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at     TIMESTAMPTZ,
    -- provider_status: the FCM HTTP status/message code. NOT a delivery guarantee.
    provider_status VARCHAR(50),
    provider_message TEXT,
    -- A provider accepting the request ≠ device delivery ≠ user opened it.
    is_provider_accepted BOOLEAN,
    error_message   TEXT
);

COMMENT ON TABLE delivery_attempts IS
    'Per-attempt records. provider_status is FCM acceptance only — NOT proof of device delivery or user open.';

CREATE INDEX delivery_attempts_job_id_idx ON delivery_attempts(job_id);

-- Tips: private sightings submitted by the public after seeing an alert.
-- Opaque receipt returned to submitter. Exact content is never public.
CREATE TABLE tips (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    alert_id            UUID NOT NULL REFERENCES alerts(id),
    -- idempotency_key: client token to prevent duplicate tip submissions on retry
    idempotency_key     UUID NOT NULL UNIQUE,
    sighting_description TEXT NOT NULL,
    approximate_location TEXT,
    contact_preference  TEXT,
    received_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE tips IS
    'Private sightings. Exact content restricted to authorized reviewers. Public only receives an opaque receipt. Claimant has no automatic access.';

CREATE INDEX tips_alert_id_idx ON tips(alert_id);

-- Audit events: immutable log of all security-relevant actions
CREATE TABLE audit_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type      VARCHAR(100) NOT NULL,
    actor_type      VARCHAR(20) NOT NULL CHECK (actor_type IN ('system', 'staff', 'session', 'public')),
    actor_id        UUID,
    organization_id UUID,
    target_type     VARCHAR(50),
    target_id       UUID,
    -- metadata is free-form JSON but MUST NOT contain case narrative or secrets.
    metadata        JSONB NOT NULL DEFAULT '{}',
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- ip_hash is a hashed IP for rate-limit audit, not raw IP storage.
    ip_hash         VARCHAR(64)
);

COMMENT ON TABLE audit_events IS
    'Immutable security audit log. metadata must never contain case narrative, return secrets, or staff notes.';

CREATE INDEX audit_events_actor_idx ON audit_events(actor_id, occurred_at);
CREATE INDEX audit_events_target_idx ON audit_events(target_type, target_id);
CREATE INDEX audit_events_type_idx ON audit_events(event_type, occurred_at);
