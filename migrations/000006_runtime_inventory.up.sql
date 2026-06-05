-- +goose Up
-- +goose StatementBegin
CREATE TABLE runtime_services (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE runtime_deployments (
    id UUID PRIMARY KEY,
    service_id UUID NOT NULL REFERENCES runtime_services(id) ON DELETE CASCADE,
    environment TEXT NOT NULL,
    git_commit TEXT NOT NULL DEFAULT '',
    build_version TEXT NOT NULL DEFAULT '',
    reported_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX runtime_deployments_service_environment_reported_idx ON runtime_deployments (service_id, environment, reported_at DESC);
CREATE INDEX runtime_deployments_environment_idx ON runtime_deployments (environment);
CREATE INDEX runtime_deployments_reported_at_idx ON runtime_deployments (reported_at DESC);

CREATE TABLE runtime_module_usages (
    id UUID PRIMARY KEY,
    deployment_id UUID NOT NULL REFERENCES runtime_deployments(id) ON DELETE CASCADE,
    module_id UUID NULL REFERENCES modules(id) ON DELETE SET NULL,
    module_name TEXT NOT NULL,
    module_version_id UUID NULL REFERENCES module_versions(id) ON DELETE SET NULL,
    version TEXT NOT NULL,
    drift_status TEXT NOT NULL,
    drift_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX runtime_module_usages_deployment_idx ON runtime_module_usages (deployment_id);
CREATE INDEX runtime_module_usages_module_idx ON runtime_module_usages (module_id);
CREATE INDEX runtime_module_usages_module_name_idx ON runtime_module_usages (module_name);
CREATE INDEX runtime_module_usages_module_version_idx ON runtime_module_usages (module_version_id);
CREATE INDEX runtime_module_usages_drift_status_idx ON runtime_module_usages (drift_status);
-- +goose StatementEnd
