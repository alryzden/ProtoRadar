-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS outbox_records;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS module_versions;
DROP TABLE IF EXISTS modules;
-- +goose StatementEnd
