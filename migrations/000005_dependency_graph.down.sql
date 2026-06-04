-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS unresolved_proto_dependencies;
DROP TABLE IF EXISTS module_dependencies;
-- +goose StatementEnd
