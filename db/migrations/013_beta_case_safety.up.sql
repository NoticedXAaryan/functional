-- Forward-only security upgrade: old short return phrases are no longer accepted.
-- Existing reports remain intact. New reports receive a 256-bit return capability.
ALTER TABLE private_sessions ADD COLUMN return_code_digest TEXT UNIQUE;
ALTER TABLE private_sessions ADD COLUMN return_expires_at TIMESTAMPTZ;
ALTER TABLE cases ADD COLUMN submission_digest TEXT;
ALTER TABLE contact_preferences ADD COLUMN time_zone TEXT NOT NULL DEFAULT 'Asia/Kolkata';
ALTER TABLE entry_points ADD COLUMN independent_organization_id UUID REFERENCES organizations(id);
COMMENT ON COLUMN private_sessions.return_code_digest IS 'SHA-256 of random 256-bit capability. Separate from the rotating short-lived token_jti.';
