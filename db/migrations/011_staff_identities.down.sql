DROP TABLE staff_identities;
-- An empty bcrypt hash cannot authenticate. Preserve externally provisioned rows.
UPDATE staff_members SET password_hash = '' WHERE password_hash IS NULL;
ALTER TABLE staff_members ALTER COLUMN password_hash SET NOT NULL;
