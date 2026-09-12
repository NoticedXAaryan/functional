DROP INDEX one_activation_per_revision;
UPDATE delivery_jobs SET status = 'FAILED' WHERE status = 'AMBIGUOUS';
ALTER TABLE delivery_jobs DROP CONSTRAINT delivery_jobs_status_check;
ALTER TABLE delivery_jobs ADD CONSTRAINT delivery_jobs_status_check CHECK
    (status IN ('QUEUED','PROCESSING','SENT','FAILED','CANCELLED','EXPIRED'));
ALTER TABLE delivery_jobs DROP COLUMN processing_started_at;
DROP INDEX managed_subscription_endpoint;
DROP INDEX subscription_area;
ALTER TABLE area_subscriptions DROP COLUMN management_secret_hash, DROP COLUMN area_id;
ALTER TABLE alert_revisions DROP COLUMN verification_reference, DROP COLUMN area_id;
ALTER TABLE alerts DROP COLUMN is_test;
DROP TABLE alert_areas;
