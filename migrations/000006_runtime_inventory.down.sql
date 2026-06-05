-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS runtime_module_usages;
DROP TABLE IF EXISTS runtime_deployments;
DROP TABLE IF EXISTS runtime_services;
-- +goose StatementEnd
