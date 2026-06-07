-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS outbox_records_published_at_idx;
DROP INDEX IF EXISTS outbox_records_claim_expiry_idx;

ALTER TABLE outbox_records
    DROP CONSTRAINT IF EXISTS outbox_records_attempts_check,
    DROP CONSTRAINT IF EXISTS outbox_records_status_check,
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS claim_expires_at,
    DROP COLUMN IF EXISTS claimed_at,
    ALTER COLUMN last_error DROP NOT NULL,
    ALTER COLUMN last_error DROP DEFAULT,
    ALTER COLUMN created_at DROP DEFAULT,
    ALTER COLUMN available_at DROP DEFAULT,
    ALTER COLUMN attempts SET DEFAULT 0,
    ALTER COLUMN status DROP DEFAULT;
-- +goose StatementEnd
