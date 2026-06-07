# REST API

The REST API is JSON over HTTP. Normal API endpoints require:

```text
Authorization: Bearer <api-token>
```

Authentication is implemented by the HTTP transport through `identity.AuthProvider`. In Community Edition, the provider validates API bearer tokens and returns an API-token `Principal`; the raw bearer token is not used as the principal subject and is not stored in request context.

When authentication succeeds, the token `last_used_at` metadata is updated best-effort. If that metadata update fails, authentication still succeeds and the server logs `api_token_mark_used_failed` with bounded token ID metadata; raw token values are not logged.

After authentication, normal API routes pass through `authorization.Authorizer` with an action/resource pair. Community Edition wires `CommunityAuthorizer`, which preserves existing Community behavior for authenticated callers. Downstream builds can replace the authorizer at bootstrap to enforce stricter policies without changing HTTP handlers.

Special route classes:

- `GET /healthz`, `GET /readyz`, and `GET /metrics` are public operational endpoints.
- `POST /api/v1/tokens` uses the bootstrap token instead of a regular API token and is intentionally bootstrap-token-only.
- All other `/api/v1` routes are authenticated and authorized.

Public and bootstrap-only routes are documented exceptions to normal authorization. Protected routes are tested with missing-auth, denying-authorizer, allowing-authorizer, and expected action/resource cases so downstream authorizers can rely on the HTTP transport invoking the boundary. Community Edition's default authorizer remains permissive for authenticated Community actions; advanced RBAC is not implemented in OSS.

Actions describe operations such as module reads, version publishing, breaking report reads, runtime inventory reporting, governance owner management, approval decisions, and edition reads. Resources describe targets such as modules, module versions, breaking reports, runtime inventory objects, governance objects, and edition metadata.

## Error Format

Errors use a stable envelope:

```json
{
  "error": {
    "code": "not_found",
    "message": "Requested resource was not found."
  }
}
```

Representative status mappings:

| HTTP | Code |
| --- | --- |
| `400` | `validation_error` or `bad_request` |
| `401` | `unauthorized` |
| `404` | `not_found` |
| `409` | `conflict` |
| `413` | `payload_too_large` |
| `422` | `unprocessable_entity` |
| `500` | `internal_error` |

Error responses do not include stack traces, raw database errors, storage credentials, or raw API tokens.

## Edition

### `GET /api/v1/edition`

Return edition, build metadata, and capability status. This endpoint requires bearer token authentication.

Response:

```json
{
  "edition": "community",
  "version": "v1.2.0",
  "commit": "abc123",
  "build_date": "2026-06-05T12:00:00Z",
  "capabilities": [
    {"name": "registry", "enabled": true},
    {"name": "breaking_checks", "enabled": true},
    {"name": "oidc_auth", "enabled": false},
    {"name": "advanced_rbac", "enabled": false}
  ]
}
```

The response does not include raw tokens, token hashes, credentials, or license data.

## Modules

### `POST /api/v1/modules`

Create a module.

Request:

```json
{
  "name": "user-api",
  "description": "User service protobuf contracts",
  "repository_url": "https://gitlab.example.com/platform/user-api"
}
```

### `GET /api/v1/modules`

List modules.

### `GET /api/v1/modules/{module}`

Get one module.

## Module Versions And Artifacts

### `POST /api/v1/modules/{module}/versions`

Publish a version. This is a multipart request with:

- `version`: version string;
- `artifact`: source archive file.

The server runs Buf, stores artifacts, extracts metadata, and updates dependency data.

### `GET /api/v1/modules/{module}/versions`

List versions for a module.

### `GET /api/v1/modules/{module}/versions/{version}`

Get version details including artifact summaries, Buf config info, lint result, and metadata summary.

Module version responses include deprecation metadata. Abbreviated example:

```json
{
  "id": "version-id",
  "module_id": "module-id",
  "version": "v1.0.0",
  "created_at": "2026-06-04T12:00:00Z",
  "deprecated_at": "2026-06-04T12:30:00Z",
  "deprecated_by": "api-token:release-bot",
  "deprecation_reason": "Use v1.2.0 instead."
}
```

For non-deprecated versions, `deprecated_at` is omitted or null and `deprecated_by`/`deprecation_reason` are empty strings.

### `GET /api/v1/modules/{module}/versions/{version}/artifact`

Download the stored source archive.

Deprecating a version does not delete stored artifacts and does not make this artifact endpoint unavailable by itself.

### `POST /api/v1/modules/{module}/versions/{version}/deprecate`

Mark a published module version as deprecated.

Request:

```json
{
  "reason": "Use v1.2.0 instead."
}
```

The `reason` field is optional. The server records:

- `deprecated_at`: server time;
- `deprecated_by`: authenticated principal subject;
- `deprecation_reason`: trimmed request reason or an empty string.

Clients must not send an actor. The route requires normal bearer-token authentication and authorization action `module_version:deprecate` on the target module version resource.

Abbreviated response:

```json
{
  "id": "version-id",
  "module_id": "module-id",
  "version": "v1.0.0",
  "created_at": "2026-06-04T12:00:00Z",
  "deprecated_at": "2026-06-04T12:30:00Z",
  "deprecated_by": "api-token:release-bot",
  "deprecation_reason": "Use v1.2.0 instead."
}
```

Repeating the request for an already deprecated version is idempotent: the server returns the existing deprecated version metadata rather than overwriting it. Unknown modules or versions return `404 not_found`.

## Descriptor Metadata

### `GET /api/v1/modules/{module}/versions/{version}/metadata`

Returns extracted protobuf descriptor metadata including files, imports, services, methods, messages, fields, enums, enum values, and summary counts.

## Breaking Checks And Reports

### `POST /api/v1/modules/{module}/breaking-checks`

Run a server-side breaking check. Multipart fields:

- `artifact`: proposed source archive;
- `against`: baseline version or `latest`;
- `target_ref`: optional branch/SHA label.

Breaking changes are normal check results. A successful check with breaking changes returns HTTP `200` with `status` set to `breaking`; it is not an API error.

### `GET /api/v1/breaking-reports/{report_id}`

Get a stored breaking report and its changes.

### `GET /api/v1/modules/{module}/breaking-reports`

List reports for a module.

### `GET /api/v1/breaking-reports/{report_id}/affected-modules`

List direct downstream modules that may be affected by a breaking report.

### `GET /api/v1/breaking-reports/{report_id}/runtime-impact`

List runtime deployments that may be affected by the breaking report based on exact base-version usage.

## GitLab Project Mapping

### `PUT /api/v1/modules/{module}/gitlab-project`

Link a ProtoRadar module to a GitLab project.

Request:

```json
{
  "gitlab_base_url": "https://gitlab.example.com",
  "gitlab_project_id": 12345,
  "gitlab_project_path": "platform/user-api"
}
```

### `GET /api/v1/modules/{module}/gitlab-project`

Get the mapping for a module.

## Dependency Graph

### `GET /api/v1/modules/{module}/dependencies`

Returns upstream dependencies, downstream consumers, and unresolved dependencies.

### `GET /api/v1/modules/{module}/affected`

Returns direct downstream consumers of a provider module.

Dependency graph is direct-only in Community v1.0.

## Runtime Inventory

### `POST /api/v1/runtime/reports`

Report deployed module versions.

Request:

```json
{
  "service_name": "billing-service",
  "environment": "production",
  "git_commit": "abc1234",
  "build_version": "demo-v1",
  "modules": [
    {"module": "user-api", "version": "v1.0.0"},
    {"module": "billing-api", "version": "v1.0.0"}
  ]
}
```

Drift statuses such as `behind_latest` and `unknown_version` are successful report results, not API errors.

If a reported module version exists and is marked deprecated, the report response includes `deprecated_version`:

```json
{
  "service_name": "billing-service",
  "environment": "production",
  "usages": [
    {
      "module": "user-api",
      "version": "v1.0.0",
      "latest_version": "v1.2.0",
      "drift_status": "deprecated_version",
      "drift_reason": "deprecated_version"
    }
  ]
}
```

### `GET /api/v1/runtime/services`

List runtime service summaries.

### `GET /api/v1/runtime/services/{service}`

Get deployments and module usages for a service.

### `GET /api/v1/runtime/environments/{environment}`

Get deployments and module usages for an environment.

### `GET /api/v1/modules/{module}/runtime-usages`

Get runtime usages of a module.

Runtime inventory responses render drift statuses calculated by the server. `drift_status` can be `up_to_date`, `behind_latest`, `unknown_version`, or `deprecated_version`.

Drift precedence is:

1. `unknown_version`
2. `deprecated_version`
3. `behind_latest`
4. `up_to_date`

Runtime inventory responses do not expose raw tokens. They expose the server-calculated drift result for a usage; module-version deprecation actor metadata is available from module version responses, not runtime usage responses.

Breaking report runtime impact responses include `impact_status` for breaking-report context and, when known, the stored runtime `drift_status` and `drift_reason` for the matching usage. Deprecated runtime usage is represented as `drift_status: "deprecated_version"`; deprecation actor metadata and raw tokens are not exposed by runtime inventory responses.

## Governance

Governance endpoints manage module ownership, approval requests, approval decisions, and governance audit reads. All endpoints require bearer token authentication. They do not expose API tokens, GitLab tokens, storage credentials, stack traces, or raw database errors.

Governance audit actors and approval decision actors are resolved from the authenticated `Principal.Subject` by default. In Community Edition this is an API-token principal; the raw bearer token is never used as the actor. Clients should omit `actor` fields and query parameters by default.

Deprecated request `actor` fields and query parameters are compatibility-only. If they differ from the authenticated principal, the server rejects them unless `governance.actor_override_enabled=true`. Enabling override lets authenticated callers record a different actor and weakens audit reliability.

Allowed governance values:

- Owner subject types: `user`, `team`.
- Owner roles: `owner`, `maintainer`.
- Approval request statuses: `pending`, `approved`, `rejected`, `cancelled`, `not_required`.
- Approval requirement types: `module_owner_approval`, `affected_consumer_approval`.
- Approval requirement statuses: `pending`, `approved`, `rejected`, `not_required`.
- Approval decisions: `approved`, `rejected`.
- Audit event types returned by audit APIs: `module_owner_added`, `module_owner_removed`, `approval_request_created`, `approval_decision_recorded`, `approval_request_status_changed`, `policy_evaluated`.

### `GET /api/v1/modules/{module}/owners`

List owners and maintainers for a module.

Response:

```json
{
  "module": "user-api",
  "owners": [
    {
      "id": "owner-1",
      "module_id": "module-1",
      "module_name": "user-api",
      "subject_type": "team",
      "subject": "platform-team",
      "role": "owner",
      "created_at": "2026-06-05T12:00:00Z",
      "updated_at": "2026-06-05T12:00:00Z"
    }
  ]
}
```

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

`subject_type` must be `user` or `team`. `role` must be `owner` or `maintainer`. Do not send `actor` by default; the server uses the authenticated principal. Optional legacy `actor` values are accepted only when they match the authenticated principal or actor override is enabled. Invalid values return HTTP `400` with error code `validation_error`; responses do not expose SQLSTATE values, CHECK constraint names, or raw database errors. Duplicate owner records return conflict.

### `DELETE /api/v1/modules/{module}/owners/{owner_id}`

Remove an owner or maintainer. The audit actor is the authenticated principal by default. Do not send `actor` by default. The optional legacy `actor` query parameter is rejected when it differs from the principal unless actor override is enabled.

Example:

```text
DELETE /api/v1/modules/user-api/owners/owner-1
```

Success returns `204 No Content`.

### `POST /api/v1/breaking-reports/{report_id}/approval-request`

Create or return the approval request for a breaking report. Passed/non-breaking reports create a `not_required` request for history. Breaking reports create a `pending` request with policy-generated requirements. This operation is idempotent per breaking report: repeated or concurrent calls return the existing approval request and do not create duplicate requirements, audit events, or outbox records.

The request body should be `{}`. Deprecated `actor` fields are compatibility-only and are rejected when they differ from the authenticated principal unless actor override is enabled.

Request:

```json
{}
```

Response:

```json
{
  "id": "approval-1",
  "module_id": "module-1",
  "module_name": "user-api",
  "breaking_report_id": "report-1",
  "target_ref": "feature/remove-field",
  "status": "pending",
  "required_approvals": 1,
  "received_approvals": 0,
  "requirements": [
    {
      "id": "requirement-1",
      "approval_request_id": "approval-1",
      "requirement_type": "module_owner_approval",
      "target_module_id": "module-1",
      "target_module_name": "user-api",
      "required_role": "owner",
      "status": "pending",
      "reason": "Breaking changes require approval from module owner.",
      "created_at": "2026-06-05T12:00:00Z",
      "updated_at": "2026-06-05T12:00:00Z"
    }
  ],
  "decisions": [],
  "created_at": "2026-06-05T12:00:00Z",
  "updated_at": "2026-06-05T12:00:00Z"
}
```

### `GET /api/v1/breaking-reports/{report_id}/approval-status`

Get approval request status, requirements, and decisions for a breaking report. Returns `404` if no approval request exists for the report.

### `POST /api/v1/approval-requests/{request_id}/requirements/{requirement_id}/approve`

Approve a pending requirement. The decision value is not user-supplied; this route records the fixed decision `approved`.

Request:

```json
{
  "comment": "Approved because consumers have been notified."
}
```

Successful approval returns the updated approval request. Decision objects include `decided_by`, which is set by the server from the effective governance actor. If all requirements are approved, the request status becomes `approved`. A requirement decision is final: repeated approve/reject, approve-after-reject, and reject-after-approve return `409 conflict` with the stable API error code `conflict`. Duplicate failed attempts do not create audit or outbox records.

Do not send `actor` by default. Deprecated `actor` fields are compatibility-only and require server actor override when they differ from the authenticated principal.

### `POST /api/v1/approval-requests/{request_id}/requirements/{requirement_id}/reject`

Reject a pending requirement. The decision value is not user-supplied; this route records the fixed decision `rejected`.

Request:

```json
{
  "comment": "Rejected because billing-api is still using this version in production."
}
```

Successful rejection returns the updated approval request with status `rejected`. Decision objects include `decided_by`, which is set by the server from the effective governance actor. Rejection is a valid governance outcome and not an API error. If the requirement already has any decision, rejection returns `409 conflict` with the stable API error code `conflict`.

Do not send `actor` by default. Deprecated `actor` fields are compatibility-only and require server actor override when they differ from the authenticated principal.

### `GET /api/v1/approval-requests/{request_id}/audit`

List governance audit events for an approval request. Optional query parameters:

- `limit`
- `offset`

Response:

```json
{
  "approval_request_id": "approval-1",
  "events": [
    {
      "id": "audit-1",
      "event_type": "approval_request_created",
      "actor": "ci-api-token",
      "module_id": "module-1",
      "module_name": "user-api",
      "approval_request_id": "approval-1",
      "breaking_report_id": "report-1",
      "payload": {"requirement_count": 1},
      "created_at": "2026-06-05T12:00:00Z"
    }
  ]
}
```

## Tokens

### `POST /api/v1/tokens`

Create an API token using the bootstrap token.

Request:

```json
{
  "name": "ci",
  "expires_at": "2026-12-31T00:00:00Z"
}
```

`expires_at` is optional and must use RFC3339 when present. The raw token is returned once.

## Health, Readiness, Metrics

### `GET /healthz`

Returns lightweight liveness:

```json
{"status":"ok"}
```

### `GET /readyz`

Checks required dependencies such as PostgreSQL:

```json
{
  "status": "ok",
  "checks": {
    "database": "ok"
  }
}
```

Returns HTTP `503` when a required dependency is unavailable.

### `GET /metrics`

Returns Prometheus text-format metrics. This endpoint is public by design in Community v1.0; protect it with network controls or a reverse proxy if needed. Do not rely on raw path labels; HTTP metrics use route templates.
