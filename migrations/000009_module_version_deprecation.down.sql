-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS module_versions_deprecated_at_idx;

ALTER TABLE module_versions
    DROP COLUMN IF EXISTS deprecation_reason,
    DROP COLUMN IF EXISTS deprecated_by,
    DROP COLUMN IF EXISTS deprecated_at;
-- +goose StatementEnd
