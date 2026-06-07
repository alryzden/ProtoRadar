-- +goose Up
-- +goose StatementBegin
-- Outbox lifecycle hardening keeps existing rows durable. Existing non-published rows remain
-- pending/retryable; invalid historical statuses must be cleaned before this migration.
UPDATE outbox_records
SET last_error = ''
WHERE last_error IS NULL;

ALTER TABLE outbox_records
    ALTER COLUMN status SET DEFAULT 'pending',
    ALTER COLUMN attempts SET DEFAULT 0,
    ALTER COLUMN available_at SET DEFAULT now(),
    ALTER COLUMN created_at SET DEFAULT now(),
    ALTER COLUMN last_error SET DEFAULT '',
    ALTER COLUMN last_error SET NOT NULL,
    ADD COLUMN claimed_at TIMESTAMPTZ NULL,
    ADD COLUMN claim_expires_at TIMESTAMPTZ NULL,
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD CONSTRAINT outbox_records_status_check CHECK (status IN ('pending', 'processing', 'published', 'failed', 'dead')),
    ADD CONSTRAINT outbox_records_attempts_check CHECK (attempts >= 0);

CREATE INDEX outbox_records_claim_expiry_idx ON outbox_records (status, claim_expires_at);
CREATE INDEX outbox_records_published_at_idx ON outbox_records (published_at) WHERE published_at IS NOT NULL;
-- Existing outbox_records_available_idx on (status, available_at, created_at) supports future
-- pending claim queries using FOR UPDATE SKIP LOCKED.
-- +goose StatementEnd
