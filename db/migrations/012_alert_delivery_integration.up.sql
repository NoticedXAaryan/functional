-- Registered areas provide a stable, explicit audience. No GPS tracking is needed.
CREATE TABLE alert_areas (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    is_test BOOLEAN NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE
);
INSERT INTO alert_areas (id, name, is_test) VALUES
    ('demo-nagar-north', '[FICTIONAL] Nagar North', TRUE),
    ('demo-nagar-south', '[FICTIONAL] Nagar South', TRUE);

ALTER TABLE alerts ADD COLUMN is_test BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE alert_revisions ADD COLUMN area_id TEXT REFERENCES alert_areas(id);
ALTER TABLE alert_revisions ADD COLUMN verification_reference TEXT;
ALTER TABLE area_subscriptions ADD COLUMN area_id TEXT REFERENCES alert_areas(id);
ALTER TABLE area_subscriptions ADD COLUMN management_secret_hash TEXT;
CREATE UNIQUE INDEX managed_subscription_endpoint ON area_subscriptions(endpoint)
    WHERE revoked_at IS NULL AND management_secret_hash IS NOT NULL;
CREATE INDEX subscription_area ON area_subscriptions(area_id, is_test) WHERE revoked_at IS NULL;
ALTER TABLE delivery_jobs ADD COLUMN processing_started_at TIMESTAMPTZ;
ALTER TABLE delivery_jobs DROP CONSTRAINT delivery_jobs_status_check;
ALTER TABLE delivery_jobs ADD CONSTRAINT delivery_jobs_status_check CHECK
    (status IN ('QUEUED','PROCESSING','SENT','FAILED','CANCELLED','EXPIRED','AMBIGUOUS'));
CREATE UNIQUE INDEX one_activation_per_revision ON outbox_events(revision_id)
    WHERE event_type = 'alert_activated' AND status = 'PENDING';
-- Old rows without a registered area are deliberately not eligible for delivery.
