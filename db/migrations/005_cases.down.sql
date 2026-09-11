ALTER TABLE private_sessions DROP CONSTRAINT IF EXISTS private_sessions_case_id_fk;
DROP TABLE IF EXISTS case_events;
DROP TABLE IF EXISTS case_access_grants;
DROP TABLE IF EXISTS contact_preferences;
DROP TABLE IF EXISTS cases;
DROP TYPE IF EXISTS case_status;
