-- +goose Up
-- +goose StatementBegin
-- Governance enum-like values are enforced with CHECK constraints. Invalid existing rows
-- must be cleaned before applying equivalent constraints to an already-populated schema.
CREATE TABLE module_owners (
    id UUID PRIMARY KEY,
    module_id UUID NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    subject_type TEXT NOT NULL CONSTRAINT module_owners_subject_type_check CHECK (subject_type IN ('user', 'team')),
    subject TEXT NOT NULL,
    role TEXT NOT NULL CONSTRAINT module_owners_role_check CHECK (role IN ('owner', 'maintainer')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (module_id, subject_type, subject, role)
);

CREATE INDEX module_owners_module_id_idx ON module_owners (module_id);
CREATE INDEX module_owners_subject_idx ON module_owners (subject_type, subject);

CREATE TABLE approval_requests (
    id UUID PRIMARY KEY,
    module_id UUID NOT NULL REFERENCES modules(id) ON DELETE CASCADE,
    breaking_report_id UUID NULL REFERENCES breaking_reports(id) ON DELETE SET NULL,
    target_ref TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CONSTRAINT approval_requests_status_check CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled', 'not_required')),
    required_approvals INTEGER NOT NULL DEFAULT 0,
    received_approvals INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX approval_requests_module_id_idx ON approval_requests (module_id);
-- Existing duplicate approval requests for the same breaking report must be cleaned before applying this migration.
CREATE UNIQUE INDEX approval_requests_breaking_report_unique_idx
    ON approval_requests (breaking_report_id)
    WHERE breaking_report_id IS NOT NULL;
CREATE INDEX approval_requests_status_idx ON approval_requests (status);

CREATE TABLE approval_requirements (
    id UUID PRIMARY KEY,
    approval_request_id UUID NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    requirement_type TEXT NOT NULL CONSTRAINT approval_requirements_type_check CHECK (requirement_type IN ('module_owner_approval', 'affected_consumer_approval')),
    target_module_id UUID NULL REFERENCES modules(id) ON DELETE SET NULL,
    target_module_name TEXT NOT NULL DEFAULT '',
    required_role TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CONSTRAINT approval_requirements_status_check CHECK (status IN ('pending', 'approved', 'rejected', 'not_required')),
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX approval_requirements_approval_request_id_idx ON approval_requirements (approval_request_id);
CREATE INDEX approval_requirements_target_module_id_idx ON approval_requirements (target_module_id);
CREATE INDEX approval_requirements_status_idx ON approval_requirements (status);

CREATE TABLE approval_decisions (
    id UUID PRIMARY KEY,
    approval_request_id UUID NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    requirement_id UUID NOT NULL REFERENCES approval_requirements(id) ON DELETE CASCADE,
    decision TEXT NOT NULL CONSTRAINT approval_decisions_decision_check CHECK (decision IN ('approved', 'rejected')),
    decided_by TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX approval_decisions_approval_request_id_idx ON approval_decisions (approval_request_id);
-- Each approval requirement records at most one final decision; decision history belongs in governance_audit_events.
CREATE UNIQUE INDEX approval_decisions_requirement_unique_idx ON approval_decisions (requirement_id);
CREATE INDEX approval_decisions_decided_by_idx ON approval_decisions (decided_by);

CREATE TABLE governance_audit_events (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL CONSTRAINT governance_audit_events_type_check CHECK (event_type IN ('module_owner_added', 'module_owner_removed', 'approval_request_created', 'approval_decision_recorded', 'approval_request_status_changed', 'policy_evaluated')),
    actor TEXT NOT NULL DEFAULT '',
    module_id UUID NULL REFERENCES modules(id) ON DELETE SET NULL,
    module_name TEXT NOT NULL DEFAULT '',
    approval_request_id UUID NULL REFERENCES approval_requests(id) ON DELETE SET NULL,
    breaking_report_id UUID NULL REFERENCES breaking_reports(id) ON DELETE SET NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX governance_audit_events_module_id_idx ON governance_audit_events (module_id);
CREATE INDEX governance_audit_events_approval_request_id_idx ON governance_audit_events (approval_request_id);
CREATE INDEX governance_audit_events_event_type_idx ON governance_audit_events (event_type);
CREATE INDEX governance_audit_events_created_at_idx ON governance_audit_events (created_at DESC);
-- +goose StatementEnd
