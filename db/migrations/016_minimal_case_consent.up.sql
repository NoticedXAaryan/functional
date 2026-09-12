-- Migration 016: Allow minimal case submissions and add in-app reply consent
-- Task 04 (Bal Setu UI): A report with only a route_type is a valid help request.
-- Children must not be blocked from sending by a mandatory narrative.
-- in_app_reply_consent: nullable; NULL = legacy record (no consent collected);
--   true = child chose "Yes, when I come back"; false = child chose "Send this without replies".
-- This does NOT change the preferred_channel contact_preference column.
-- preferred_channel=no_contact is preserved for legacy records and when reply=false.

-- 1. Allow empty account_text (was NOT NULL, now empty string is valid).
--    A route_type alone constitutes a valid help signal.
ALTER TABLE cases
    ALTER COLUMN account_text DROP NOT NULL,
    ALTER COLUMN account_text SET DEFAULT '';

UPDATE cases SET account_text = '' WHERE account_text IS NULL;

-- 2. Add in_app_reply_consent to contact_preferences.
--    NULL = consent not captured (legacy). true/false = child's explicit choice.
ALTER TABLE contact_preferences
    ADD COLUMN IF NOT EXISTS in_app_reply_consent BOOLEAN;

COMMENT ON COLUMN contact_preferences.in_app_reply_consent IS
    'Explicit in-app reply consent from the child. NULL = legacy record (consent not captured). '
    'true = child wants in-app replies when they return. false = child chose no replies. '
    'Staff must not start a reply-style conversation without explicit true consent.';
