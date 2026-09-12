ALTER TABLE entry_points DROP COLUMN independent_organization_id;
ALTER TABLE contact_preferences DROP COLUMN time_zone;
ALTER TABLE cases DROP COLUMN submission_digest;
ALTER TABLE private_sessions DROP COLUMN return_expires_at;
ALTER TABLE private_sessions DROP COLUMN return_code_digest;
