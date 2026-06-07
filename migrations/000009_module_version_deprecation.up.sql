-- +goose Up
-- +goose StatementBegin
ALTER TABLE module_versions
    ADD COLUMN deprecated_at TIMESTAMPTZ NULL,
    ADD COLUMN deprecated_by TEXT NOT NULL DEFAULT '',
    ADD COLUMN deprecation_reason TEXT NOT NULL DEFAULT '';

CREATE INDEX module_versions_deprecated_at_idx
    ON module_versions (deprecated_at)
    WHERE deprecated_at IS NOT NULL;
-- +goose StatementEnd
