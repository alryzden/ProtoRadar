-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS proto_enum_values;
DROP TABLE IF EXISTS proto_enums;
DROP TABLE IF EXISTS proto_fields;
DROP TABLE IF EXISTS proto_messages;
DROP TABLE IF EXISTS proto_methods;
DROP TABLE IF EXISTS proto_services;
DROP TABLE IF EXISTS proto_imports;
DROP TABLE IF EXISTS proto_files;
DROP TABLE IF EXISTS module_version_buf_configs;

ALTER TABLE artifacts
    DROP CONSTRAINT IF EXISTS artifacts_module_version_kind_key;

ALTER TABLE artifacts
    DROP CONSTRAINT IF EXISTS artifacts_kind_check;

ALTER TABLE artifacts
    DROP COLUMN IF EXISTS kind;

ALTER TABLE artifacts
    ADD CONSTRAINT artifacts_module_version_id_key UNIQUE (module_version_id);
-- +goose StatementEnd
