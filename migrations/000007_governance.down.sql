-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS approval_decisions_requirement_unique_idx;
DROP INDEX IF EXISTS approval_requests_breaking_report_unique_idx;
DROP TABLE IF EXISTS governance_audit_events;
DROP TABLE IF EXISTS approval_decisions;
DROP TABLE IF EXISTS approval_requirements;
DROP TABLE IF EXISTS approval_requests;
DROP TABLE IF EXISTS module_owners;
-- +goose StatementEnd
