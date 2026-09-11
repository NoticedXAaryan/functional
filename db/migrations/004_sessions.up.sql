-- Migration 004: Private sessions and return secrets
-- A session is created when a user scans a QR code and begins a flow.
-- mode: one_time | with_return_access
-- The return_secret_hash is a bcrypt hash of the BIP-39 mnemonic phrase.
-- The plaintext is NEVER stored — it is shown once and discarded.

CREATE TABLE private_sessions (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entry_point_id      UUID REFERENCES entry_points(id),
    mode                VARCHAR(20) NOT NULL CHECK (mode IN ('one_time', 'with_return_access')),
    language            VARCHAR(10) NOT NULL DEFAULT 'en',
    -- JWT credential for this session. Stored as a reference only.
    -- The actual JWT is signed with SESSION_SECRET.
    token_jti           UUID NOT NULL UNIQUE,
    -- Bcrypt hash of the BIP-39 words. Only set for with_return_access sessions.
    -- NULL for one_time sessions.
    return_secret_hash  VARCHAR(255),
    -- attempt_count tracks brute-force attempts on session-access endpoint.
    access_attempt_count INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '15 minutes'),
    -- revoked_at is set on explicit exit or inactivity expiry.
    -- A revoked session cannot be used even if expires_at has not passed.
    revoked_at          TIMESTAMPTZ,
    -- case_id is set once a case is submitted through this session.
    case_id             UUID  -- FK added after cases table exists (migration 005)
);

COMMENT ON TABLE private_sessions IS
    'One-time or return-access sessions. Return secrets are stored only as bcrypt hashes. Revocation is immediate and enforced before every authenticated request.';
COMMENT ON COLUMN private_sessions.return_secret_hash IS
    'bcrypt hash of the BIP-39 mnemonic. Plaintext never stored. Rate-limited brute force via access_attempt_count.';

CREATE INDEX private_sessions_token_jti_idx ON private_sessions(token_jti);
CREATE INDEX private_sessions_case_id_idx ON private_sessions(case_id);
