ALTER TABLE alerts DROP CONSTRAINT IF EXISTS alerts_active_revision_id_fk;
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS alert_approvals;
DROP TABLE IF EXISTS alert_revisions;
DROP TABLE IF EXISTS alerts;
DROP TYPE IF EXISTS alert_status;
