-- Bind a verified identity-provider subject to an explicitly provisioned staff account.
-- Email matches and roles in external JWTs must not provision access automatically.
CREATE TABLE staff_identities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    staff_id UUID NOT NULL REFERENCES staff_members(id),
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    UNIQUE (issuer, subject)
);
CREATE INDEX staff_identities_staff_idx ON staff_identities(staff_id);
-- Provider-only accounts do not need a local password; demo login denies missing hashes.
ALTER TABLE staff_members ALTER COLUMN password_hash DROP NOT NULL;
