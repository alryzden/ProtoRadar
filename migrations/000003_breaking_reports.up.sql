-- +goose Up
-- +goose StatementBegin
CREATE TABLE breaking_reports (
    id UUID PRIMARY KEY,
    module_id UUID NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    base_version_id UUID NOT NULL REFERENCES module_versions(id) ON DELETE RESTRICT,
    target_ref TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('passed', 'breaking', 'failed')),
    change_count INTEGER NOT NULL DEFAULT 0,
    raw_output TEXT NOT NULL DEFAULT '',
    human_summary TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE breaking_changes (
    id UUID PRIMARY KEY,
    report_id UUID NOT NULL REFERENCES breaking_reports(id) ON DELETE CASCADE,
    category TEXT NOT NULL DEFAULT '',
    file_path TEXT NOT NULL DEFAULT '',
    package_name TEXT NOT NULL DEFAULT '',
    symbol TEXT NOT NULL DEFAULT '',
    rule_id TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'error',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX breaking_reports_module_created_at_idx ON breaking_reports (module_id, created_at DESC);
CREATE INDEX breaking_reports_base_version_idx ON breaking_reports (base_version_id);
CREATE INDEX breaking_reports_status_idx ON breaking_reports (status);
CREATE INDEX breaking_changes_report_idx ON breaking_changes (report_id);
CREATE INDEX breaking_changes_file_path_idx ON breaking_changes (file_path);
CREATE INDEX breaking_changes_symbol_idx ON breaking_changes (symbol);
-- +goose StatementEnd
