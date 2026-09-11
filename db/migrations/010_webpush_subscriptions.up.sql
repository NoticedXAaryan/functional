-- Migration 010: Update area_subscriptions for Web Push (VAPID) instead of FCM tokens.
--
-- The FCM token field is renamed to 'endpoint' to hold the browser Push API
-- subscription endpoint URL. The auth and p256dh keys from the PushSubscription
-- are stored alongside it for payload encryption (RFC 8291 Message Encryption).
--
-- Existing rows with FCM tokens are not migrated (demo data only).
ALTER TABLE area_subscriptions
    RENAME COLUMN fcm_token TO endpoint;

ALTER TABLE area_subscriptions
    ADD COLUMN IF NOT EXISTS auth       TEXT,
    ADD COLUMN IF NOT EXISTS p256dh     TEXT;

COMMENT ON COLUMN area_subscriptions.endpoint IS
    'Browser Push API subscription endpoint URL (RFC 8030). NOT an FCM registration token.';
COMMENT ON COLUMN area_subscriptions.auth IS
    'auth secret from PushSubscription.getKey("auth") — base64url encoded. Required for payload encryption.';
COMMENT ON COLUMN area_subscriptions.p256dh IS
    'P-256 Diffie-Hellman public key from PushSubscription.getKey("p256dh") — base64url encoded.';
