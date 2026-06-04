-- +goose Up
-- +goose StatementBegin
CREATE TABLE module_dependencies (
    id UUID PRIMARY KEY,
    consumer_module_id UUID NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    consumer_module_version_id UUID NOT NULL REFERENCES module_versions(id) ON DELETE CASCADE,
    provider_module_id UUID NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    provider_module_version_id UUID NOT NULL REFERENCES module_versions(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    import_path TEXT NOT NULL DEFAULT '',
    referenced_package TEXT NOT NULL DEFAULT '',
    referenced_symbol TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT module_dependencies_no_self_edge CHECK (consumer_module_id <> provider_module_id)
);

CREATE UNIQUE INDEX module_dependencies_unique_edge_idx ON module_dependencies (
    consumer_module_version_id,
    provider_module_version_id,
    source,
    import_path,
    referenced_symbol
);

CREATE INDEX module_dependencies_consumer_module_idx ON module_dependencies (consumer_module_id);
CREATE INDEX module_dependencies_provider_module_idx ON module_dependencies (provider_module_id);
CREATE INDEX module_dependencies_consumer_module_version_idx ON module_dependencies (consumer_module_version_id);
CREATE INDEX module_dependencies_provider_module_version_idx ON module_dependencies (provider_module_version_id);

CREATE TABLE unresolved_proto_dependencies (
    id UUID PRIMARY KEY,
    module_id UUID NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    module_version_id UUID NOT NULL REFERENCES module_versions(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    import_path TEXT NOT NULL DEFAULT '',
    referenced_package TEXT NOT NULL DEFAULT '',
    referenced_symbol TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX unresolved_proto_dependencies_module_idx ON unresolved_proto_dependencies (module_id);
CREATE INDEX unresolved_proto_dependencies_module_version_idx ON unresolved_proto_dependencies (module_version_id);
CREATE INDEX unresolved_proto_dependencies_import_path_idx ON unresolved_proto_dependencies (import_path);
CREATE INDEX unresolved_proto_dependencies_referenced_symbol_idx ON unresolved_proto_dependencies (referenced_symbol);
-- +goose StatementEnd
