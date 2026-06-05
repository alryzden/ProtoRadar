# REST API

The public REST API is JSON over HTTP. Protected API endpoints require:

```text
Authorization: Bearer <api-token>
```

`POST /api/v1/tokens` uses the bootstrap token instead of a regular API token.

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

### `GET /api/v1/modules/{module}/versions/{version}/artifact`

Download the stored source archive.

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

### `GET /api/v1/runtime/services`

List runtime service summaries.

### `GET /api/v1/runtime/services/{service}`

Get deployments and module usages for a service.

### `GET /api/v1/runtime/environments/{environment}`

Get deployments and module usages for an environment.

### `GET /api/v1/modules/{module}/runtime-usages`

Get runtime usages of a module.

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

Returns Prometheus text-format metrics. Do not rely on raw path labels; HTTP metrics use route templates.
