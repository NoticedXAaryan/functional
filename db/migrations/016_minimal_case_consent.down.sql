-- Rollback migration 016: Restore NOT NULL on account_text, remove in_app_reply_consent.
-- WARNING: This will fail if any row has empty account_text when re-adding NOT NULL.
-- Run only after confirming all rows have non-empty account_text.

ALTER TABLE contact_preferences
    DROP COLUMN IF EXISTS in_app_reply_consent;

-- Restore the NOT NULL constraint (will error if any empty account_text exists)
ALTER TABLE cases
    ALTER COLUMN account_text DROP DEFAULT,
    ALTER COLUMN account_text SET NOT NULL;
