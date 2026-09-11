-- Rollback: restore FCM token column name
ALTER TABLE area_subscriptions
    DROP COLUMN IF EXISTS auth,
    DROP COLUMN IF EXISTS p256dh;

ALTER TABLE area_subscriptions
    RENAME COLUMN endpoint TO fcm_token;
