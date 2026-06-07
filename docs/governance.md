# Governance Layer

## Overview

The Governance Layer moves ProtoRadar from reporting protobuf breaking changes to controlling them through ownership, built-in policies, approval requests, approval decisions, and an audit trail.

Community governance is intentionally simple. It uses module owners and maintainers, deterministic built-in policy rules, authenticated API-token principals as audit actors, REST/CLI workflows, GitLab MR status display, and read-only Web UI visibility. Phase 11 wraps the default approval behavior in `ApprovalWorkflow` so downstream builds can replace workflow behavior later. It does not add OIDC, LDAP, advanced RBAC, GitLab group sync, license management, or a custom policy DSL.

## Effective Governance Actor

State-changing governance operations record an effective actor. In Community Edition, the HTTP transport authenticates the bearer API token and creates an API-token `Principal`. Governance handlers use that authenticated `Principal.Subject` as the default audit actor and decision actor.

Important details:

- Raw API token values are never used as actor strings.
- Clients should omit `actor` by default.
- REST `actor` request fields, query parameters, CLI `--actor`, and GitLab MR `--governance-actor` are deprecated compatibility inputs.
- When `governance.actor_override_enabled=false`, which is the default, the server rejects spoofed actor values that differ from the authenticated principal.
- When `governance.actor_override_enabled=true`, authenticated callers may record a different actor for migration compatibility. This weakens audit reliability and should not be used as a security control.

Community actor identity is only as strong as the API-token principal. There is no OIDC, LDAP, GitLab human identity mapping, advanced RBAC, or GitLab group sync in the OSS Community build.

## Module Owners And Maintainers

A module owner record identifies who can approve governance requirements for a module.

Fields:

- `subject_type`: `user` or `team`.
- `subject`: user or team name, for example `alice` or `platform-team`.
- `role`: `owner` or `maintainer`.

Owners are the default required role for Phase 10 approval requirements. Maintainers can approve when `governance.allow_maintainer_approval` is enabled.

CLI examples:

```sh
protoradar module owners list user-api

protoradar module owners add user-api \
  --subject-type team \
  --subject platform-team \
  --role owner

protoradar module owners remove user-api \
  --owner-id <owner_id>
```

API examples:

```sh
curl -sS \
  -H "Authorization: Bearer $PROTORADAR_TOKEN" \
  http://localhost:8080/api/v1/modules/user-api/owners
```

```sh
curl -sS \
  -H "Authorization: Bearer $PROTORADAR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"subject_type":"team","subject":"platform-team","role":"owner"}' \
  http://localhost:8080/api/v1/modules/user-api/owners
```

```sh
curl -sS \
  -X DELETE \
  -H "Authorization: Bearer $PROTORADAR_TOKEN" \
  'http://localhost:8080/api/v1/modules/user-api/owners/<owner_id>'
```

## Basic Built-In Policies

Phase 10 includes deterministic built-in policy logic. The evaluator is application logic and does not query databases, call HTTP APIs, publish events, or depend on infrastructure.

Rules:

- Additive or non-breaking changes do not require approval. A passed breaking report creates or returns `not_required` approval status when an approval request is requested.
- Breaking changes require approval from an owner of the changed module.
- Breaking changes that affect production-used consumer modules require affected consumer approval for each production-used affected module.
- Missing owners do not fail request creation and do not auto-approve. They create visible pending requirements with a warning reason such as `No owners are configured for this module.`
- Duplicate signals for the same target module and requirement type are deduplicated.

Production-like environments are configured with `governance.production_environments`. Defaults are `production,prod`.

Deprecated runtime usage is runtime inventory drift, not a separate Community governance policy trigger. If a breaking report runtime impact row also has `drift_status: "deprecated_version"`, governance can display that context through runtime impact, but Community does not automatically create approval requirements, Slack/email alerts, or enterprise escalation workflows because a service is using a deprecated version.

## Allowed Governance Values

Governance values are validated by domain/usecase logic and protected by PostgreSQL CHECK constraints.

Allowed values:

- Owner subject types: `user`, `team`.
- Owner roles: `owner`, `maintainer`.
- Approval request statuses: `pending`, `approved`, `rejected`, `cancelled`, `not_required`.
- Approval requirement types: `module_owner_approval`, `affected_consumer_approval`.
- Approval requirement statuses: `pending`, `approved`, `rejected`, `not_required`.
- Approval decisions: `approved`, `rejected`.
- Audit event types: `module_owner_added`, `module_owner_removed`, `approval_request_created`, `approval_decision_recorded`, `approval_request_status_changed`, `policy_evaluated`.

Invalid owner `subject_type` or `role` values sent to the REST API return HTTP `400` with the stable error code `validation_error`. Responses do not include SQLSTATE values, CHECK constraint names, or raw database errors. The CLI validates `module owners add --subject-type` and `--role` before sending a request and exits `2` for invalid values.

## Approval Requests

An approval request records the governance state for a breaking report. It is created explicitly through the REST API, CLI, or by the GitLab MR bot when governance mode is enabled and no request exists yet.

Approval request statuses:

- `pending`: one or more requirements still need decisions.
- `approved`: all requirements are approved.
- `rejected`: at least one requirement was rejected.
- `cancelled`: reserved for future cancellation workflows.
- `not_required`: no approval is required by policy.

Approval requests are linked to breaking reports when created for a breaking report. Creation is idempotent per breaking report: the first request creates the approval request and requirements, and later or concurrent calls for the same breaking report return the existing request. Duplicate calls do not create duplicate requirements, audit events, or outbox records.

## Approval Decisions

Approval decisions are immutable records against a specific approval requirement.

Supported decisions:

- `approved`: marks a pending requirement approved.
- `rejected`: marks a pending requirement rejected and marks the request rejected.

Decision inputs:

- `comment`: optional explanation.

Community MVP authorization uses the authenticated principal subject for simple actor matching against configured module owners and maintainers. If `governance.allow_maintainer_approval=true`, maintainers can approve or reject requirements that target their module. A requirement can have only one final decision: repeated approve/reject, approve-after-reject, and reject-after-approve return conflict. Failed duplicate decision attempts do not create audit events or outbox records. Approval responses include `decided_by`; clients should display that server-returned value instead of guessing locally.

Legacy REST/CLI `actor` override values are rejected by default when they differ from the authenticated principal. Set `governance.actor_override_enabled=true` only for migration compatibility; it allows authenticated callers to record a different actor and weakens audit reliability.

## Governance Audit Trail

The governance audit trail is an internal product audit log stored in PostgreSQL. It is separate from the transactional outbox.

Audit events are appended through the Phase 11 `AuditSink` boundary and remain transaction-compatible with governance state changes. Current event types include:

- `module_owner_added`
- `module_owner_removed`
- `approval_request_created`
- `approval_decision_recorded`
- `approval_request_status_changed`
- `policy_evaluated` when useful for policy evaluation history

Audit payloads contain product metadata for inspection. They must not contain API tokens, GitLab tokens, S3 credentials, or other secrets.

## Transactional Outbox

Governance usecases write outbox records in the same database transaction as governance state and audit changes. Usecases never publish directly to Kafka/Sarama.

Phase 10 integration events include:

- `ApprovalRequestCreated`
- `ApprovalDecisionRecorded`
- `ModuleOwnerAdded`
- `ModuleOwnerRemoved`
- `ApprovalRequestStatusChanged`

The durable outbox remains transport-neutral. Kafka routing and publishing, if enabled later, belong outside domain and usecases.

## GitLab MR Integration

`protoradar gitlab mr-check` adds a Governance section to merge request comments when approval status is available or governance mode creates a request.

The section shows:

- overall approval status;
- requirements;
- missing owner warnings;
- decisions when present.

Governance mode is controlled with:

```sh
protoradar gitlab mr-check --governance=true ...
```

Behavior:

- `--governance=false` keeps old compatibility semantics: passed checks exit `0`, breaking checks exit `1`, operational errors exit `2`.
- `--governance=true` allows approved breaking changes to pass.
- Passed or `not_required` reports exit `0`.
- Breaking plus `approved` exits `0` and sets commit status success.
- Breaking plus `pending` exits `1` and sets commit status failed.
- Breaking plus `rejected` exits `1` and sets commit status failed.
- Governance/API/tool/internal errors in governance mode exit `2`.

The MR bot fetches governance through the ProtoRadar REST API. It does not compute policies in the renderer.

When the MR bot creates an approval request in CI, the actor is the authenticated `PROTORADAR_TOKEN` principal on the ProtoRadar server. The Community MR bot does not map GitLab usernames, GitLab groups, approvers, or merge request authors to governance actors. MR comments display governance status, requirements, and decisions returned by the server, including server-returned `decided_by` values when decisions exist.

## CLI Usage

Owners:

```sh
protoradar module owners list user-api

protoradar module owners add user-api \
  --subject-type user \
  --subject alice \
  --role owner

protoradar module owners remove user-api \
  --owner-id <owner_id>
```

Approvals:

```sh
protoradar approvals request \
  --report-id <breaking_report_id>

protoradar approvals status \
  --report-id <breaking_report_id>

protoradar approvals approve <requirement_id> \
  --request-id <approval_request_id> \
  --comment "Consumers have been notified."

protoradar approvals reject <requirement_id> \
  --request-id <approval_request_id> \
  --comment "billing-api still uses this version in production."
```

A successful reject command exits `0`; rejection is a valid governance decision, not a CLI failure.

## API Usage

All governance endpoints require bearer token authentication.

### `GET /api/v1/modules/{module}/owners`

List owners and maintainers for a module.

### `POST /api/v1/modules/{module}/owners`

Add an owner or maintainer.

Request:

```json
{
  "subject_type": "team",
  "subject": "platform-team",
  "role": "owner"
}
```

### `DELETE /api/v1/modules/{module}/owners/{owner_id}`

Remove an owner or maintainer. The audit actor is the authenticated principal by default.

Example:

```text
DELETE /api/v1/modules/user-api/owners/owner-1
```

### `POST /api/v1/breaking-reports/{report_id}/approval-request`

Create or return the approval request for a breaking report. The operation is idempotent for the same `report_id`; repeated or concurrent calls return the existing request and do not duplicate requirements, audit events, or outbox records.

Request:

```json
{}
```

### `GET /api/v1/breaking-reports/{report_id}/approval-status`

Get approval status, requirements, and decisions for a breaking report.

### `POST /api/v1/approval-requests/{request_id}/requirements/{requirement_id}/approve`

Approve a pending requirement. The first decision for a requirement wins. Repeated approval, repeated rejection, approve-after-reject, and reject-after-approve return `409 conflict`.

Request:

```json
{
  "comment": "Approved because consumers have been notified."
}
```

### `POST /api/v1/approval-requests/{request_id}/requirements/{requirement_id}/reject`

Reject a pending requirement. Rejection is a valid governance decision and exits/succeeds like approval when it is the first decision. If the requirement already has a decision, the API returns `409 conflict`.

Request:

```json
{
  "comment": "Rejected because billing-api is still using this version in production."
}
```

### `GET /api/v1/approval-requests/{request_id}/audit`

List governance audit events for an approval request. Optional query parameters:

- `limit`
- `offset`

## Web UI

The Basic Web UI is read-only for Phase 10 governance.

Pages:

- `/ui/modules/{module}` shows owners and maintainers.
- `/ui/breaking-reports/{report_id}` shows approval status, approval request ID, requirements, decisions, and missing owner warnings when an approval request exists.
- `/ui/approval-requests/{request_id}` shows request status, module, breaking report link, requirements, decisions, and audit trail.

The UI does not include approve/reject forms in Phase 10.

## Community Limitations

Phase 10 Community governance intentionally avoids enterprise identity and policy systems.

Current limitations:

- Simple API-token-principal actor identity only.
- No OIDC or LDAP.
- No advanced RBAC.
- No GitLab group or team sync.
- No advanced or custom policy DSL.
- No complex approval workflow engine.
- No approval buttons/forms in the Web UI.
- No transitive dependency governance policy yet.
- No SemVer-range runtime impact policy yet.

These are candidates for later private/downstream work.

See [Enterprise Boundary](enterprise-boundary.md) for identity, authorization, audit, workflow, capability, and edition extension points.
