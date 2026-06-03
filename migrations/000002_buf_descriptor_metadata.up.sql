-- +goose Up
-- +goose StatementBegin
ALTER TABLE artifacts
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'source_archive';

ALTER TABLE artifacts
    ADD CONSTRAINT artifacts_kind_check CHECK (kind IN ('source_archive', 'buf_image'));

ALTER TABLE artifacts
    DROP CONSTRAINT IF EXISTS artifacts_module_version_id_key;

ALTER TABLE artifacts
    ADD CONSTRAINT artifacts_module_version_kind_key UNIQUE (module_version_id, kind);

CREATE TABLE module_version_buf_configs (
    id UUID PRIMARY KEY,
    module_version_id UUID NOT NULL UNIQUE REFERENCES module_versions(id) ON DELETE CASCADE,
    buf_yaml_present BOOLEAN NOT NULL,
    buf_lock_present BOOLEAN NOT NULL,
    buf_yaml_digest TEXT NOT NULL DEFAULT '',
    buf_lock_digest TEXT NOT NULL DEFAULT '',
    module_paths JSONB NOT NULL DEFAULT '[]',
    deps JSONB NOT NULL DEFAULT '[]',
    lint_enabled BOOLEAN NOT NULL DEFAULT false,
    breaking_config_present BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE proto_files (
    id UUID PRIMARY KEY,
    module_version_id UUID NOT NULL REFERENCES module_versions(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    package_name TEXT NOT NULL DEFAULT '',
    syntax TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (module_version_id, path)
);

CREATE TABLE proto_imports (
    id UUID PRIMARY KEY,
    proto_file_id UUID NOT NULL REFERENCES proto_files(id) ON DELETE CASCADE,
    import_path TEXT NOT NULL,
    is_public BOOLEAN NOT NULL DEFAULT false,
    is_weak BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE proto_services (
    id UUID PRIMARY KEY,
    module_version_id UUID NOT NULL REFERENCES module_versions(id) ON DELETE CASCADE,
    proto_file_id UUID NOT NULL REFERENCES proto_files(id) ON DELETE CASCADE,
    package_name TEXT NOT NULL,
    name TEXT NOT NULL,
    full_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (module_version_id, full_name)
);

CREATE TABLE proto_methods (
    id UUID PRIMARY KEY,
    service_id UUID NOT NULL REFERENCES proto_services(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    input_type TEXT NOT NULL,
    output_type TEXT NOT NULL,
    client_streaming BOOLEAN NOT NULL DEFAULT false,
    server_streaming BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (service_id, name)
);

CREATE TABLE proto_messages (
    id UUID PRIMARY KEY,
    module_version_id UUID NOT NULL REFERENCES module_versions(id) ON DELETE CASCADE,
    proto_file_id UUID NOT NULL REFERENCES proto_files(id) ON DELETE CASCADE,
    parent_message_id UUID NULL REFERENCES proto_messages(id) ON DELETE CASCADE,
    package_name TEXT NOT NULL,
    name TEXT NOT NULL,
    full_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (module_version_id, full_name)
);

CREATE TABLE proto_fields (
    id UUID PRIMARY KEY,
    message_id UUID NOT NULL REFERENCES proto_messages(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    number INTEGER NOT NULL,
    type TEXT NOT NULL,
    type_name TEXT NOT NULL DEFAULT '',
    label TEXT NOT NULL DEFAULT '',
    json_name TEXT NOT NULL DEFAULT '',
    oneof_name TEXT NOT NULL DEFAULT '',
    is_repeated BOOLEAN NOT NULL DEFAULT false,
    is_map BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (message_id, number)
);

CREATE TABLE proto_enums (
    id UUID PRIMARY KEY,
    module_version_id UUID NOT NULL REFERENCES module_versions(id) ON DELETE CASCADE,
    proto_file_id UUID NOT NULL REFERENCES proto_files(id) ON DELETE CASCADE,
    package_name TEXT NOT NULL,
    name TEXT NOT NULL,
    full_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (module_version_id, full_name)
);

CREATE TABLE proto_enum_values (
    id UUID PRIMARY KEY,
    enum_id UUID NOT NULL REFERENCES proto_enums(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    number INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (enum_id, number)
);

CREATE INDEX proto_files_module_version_idx ON proto_files (module_version_id);
CREATE INDEX proto_imports_proto_file_idx ON proto_imports (proto_file_id);
CREATE INDEX proto_imports_import_path_idx ON proto_imports (import_path);
CREATE INDEX proto_services_module_version_idx ON proto_services (module_version_id);
CREATE INDEX proto_methods_service_idx ON proto_methods (service_id);
CREATE INDEX proto_messages_module_version_idx ON proto_messages (module_version_id);
CREATE INDEX proto_fields_message_idx ON proto_fields (message_id);
CREATE INDEX proto_enums_module_version_idx ON proto_enums (module_version_id);
CREATE INDEX proto_enum_values_enum_idx ON proto_enum_values (enum_id);
-- +goose StatementEnd
