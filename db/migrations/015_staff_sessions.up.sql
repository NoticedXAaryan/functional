-- Server-side staff sessions for HttpOnly cookie auth.
-- Replaces in-memory JWT-only staff auth that was lost on page refresh.
-- Private child sessions (private_sessions table) remain unchanged.

CREATE TABLE staff_sessions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    staff_id        UUID NOT NULL REFERENCES staff_members(id),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    role            VARCHAR(50) NOT NULL,
    csrf_token      VARCHAR(64) NOT NULL,
    ip_address      INET,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '8 hours'),
    last_active_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at      TIMESTAMPTZ
);

CREATE INDEX staff_sessions_staff_idx ON staff_sessions(staff_id) WHERE revoked_at IS NULL;
CREATE INDEX staff_sessions_expiry_idx ON staff_sessions(expires_at) WHERE revoked_at IS NULL;

COMMENT ON TABLE staff_sessions IS
    'Server-side staff sessions. Session ID stored in HttpOnly cookie. '
    'CSRF uses double-submit cookie pattern. Sliding expiry capped at 24h from creation.';
