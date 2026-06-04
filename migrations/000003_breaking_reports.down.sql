-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS breaking_changes;
DROP TABLE IF EXISTS breaking_reports;
-- +goose StatementEnd
