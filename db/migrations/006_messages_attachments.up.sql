-- Migration 006: Messages and attachments
-- Messages are the secure conversation between reporter and responder.
-- Attachments are stored in encrypted object storage; this table holds metadata only.

CREATE TABLE case_messages (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    case_id         UUID NOT NULL REFERENCES cases(id),
    sender_type     VARCHAR(20) NOT NULL CHECK (sender_type IN ('reporter', 'responder', 'system')),
    sender_id       UUID,  -- staff_id for responder messages; NULL for reporter/system
    body            TEXT NOT NULL,
    -- is_staff_note: if true, hidden from the reporter's child-safe view
    is_staff_note   BOOLEAN NOT NULL DEFAULT FALSE,
    sent_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE case_messages IS
    'Secure messages on a case. is_staff_note=true messages are never exposed through the child-safe view endpoint.';

CREATE INDEX case_messages_case_id_idx ON case_messages(case_id, sent_at);

CREATE TABLE attachments (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    case_id         UUID NOT NULL REFERENCES cases(id),
    uploader_type   VARCHAR(20) NOT NULL CHECK (uploader_type IN ('reporter', 'responder')),
    uploader_id     UUID,
    -- object_key is the encrypted storage path. Never a public URL.
    object_key      VARCHAR(512) NOT NULL UNIQUE,
    mime_type       VARCHAR(100) NOT NULL,
    size_bytes      BIGINT,
    -- access_level controls who can retrieve a signed URL for this attachment
    access_level    VARCHAR(30) NOT NULL DEFAULT 'restricted_staff'
        CHECK (access_level IN ('restricted_staff', 'authorized_reviewer', 'public_derivative')),
    uploaded_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE attachments IS
    'Attachment metadata only. Files live in encrypted object storage. Short-lived signed URLs are generated per request with access_level checks.';

CREATE INDEX attachments_case_id_idx ON attachments(case_id);
