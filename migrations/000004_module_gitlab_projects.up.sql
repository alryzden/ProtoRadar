-- +goose Up
-- +goose StatementBegin
CREATE TABLE module_gitlab_projects (
    id UUID PRIMARY KEY,
    module_id UUID NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    gitlab_base_url TEXT NOT NULL DEFAULT '',
    gitlab_project_id BIGINT NOT NULL,
    gitlab_project_path TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (module_id),
    UNIQUE (gitlab_base_url, gitlab_project_id)
);

CREATE INDEX module_gitlab_projects_module_id_idx ON module_gitlab_projects (module_id);
CREATE INDEX module_gitlab_projects_gitlab_project_idx ON module_gitlab_projects (gitlab_base_url, gitlab_project_id);
CREATE INDEX module_gitlab_projects_gitlab_project_path_idx ON module_gitlab_projects (gitlab_project_path);
-- +goose StatementEnd
