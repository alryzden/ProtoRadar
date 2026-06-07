# Runtime Contract Inventory

## Overview

Runtime inventory lets ProtoRadar answer two related questions:

- which protobuf module versions exist in the registry;
- which protobuf module versions are actually running in each environment.

Services or deployment pipelines report the module versions they deployed. ProtoRadar stores each report as a deployment snapshot, calculates drift from the latest known contract version, and can show runtime impact for breaking changes.

## Deprecated Module Versions

A module version can be marked deprecated when the version should no longer be used by runtime services. Deprecation is metadata on the published module version:

- it does not delete module version rows;
- it does not delete stored source or Buf image artifacts;
- it does not make the artifact unavailable for download by itself;
- it records `deprecated_at`, `deprecated_by`, and `deprecation_reason` on module version API responses.

Runtime services can still report deprecated versions. When a reported module/version exists in ProtoRadar and that version is deprecated, runtime inventory reports the usage with `drift_status: "deprecated_version"`.

Deprecate a version with the CLI:

```sh
protoradar module version deprecate user-api v1.0.0 \
  --reason "Use v1.2.0 instead."
```

Or through the REST API:

```http
POST /api/v1/modules/user-api/versions/v1.0.0/deprecate
```

```json
{
  "reason": "Use v1.2.0 instead."
}
```

The endpoint requires normal bearer-token authentication and authorization for `module_version:deprecate`. The server sets `deprecated_by` from the authenticated principal; clients do not send an actor.

## Reporting From CI

A deployment job can report inventory after a successful deploy:

```sh
protoradar runtime report \
  --service billing-service \
  --environment production \
  --git-commit "$CI_COMMIT_SHA" \
  --build-version "$CI_COMMIT_TAG" \
  --module user-api@v1.2.0
```

The CLI reads ProtoRadar auth from:

```text
PROTORADAR_SERVER_URL
PROTORADAR_TOKEN
```

Store `PROTORADAR_TOKEN` as a masked CI/CD variable. Do not echo or commit token values.

## Reporting From File

Runtime reports can also be stored in a file:

```yaml
service_name: billing-service
environment: production
git_commit: abc1234
build_version: 2026.06.04-15
modules:
  - module: user-api
    version: v1.2.0
  - module: billing-api
    version: v1.4.0
```

Report it with:

```sh
protoradar runtime report --from-file protoradar-runtime.yaml
```

When `--from-file` is used, explicit flags override file values. Repeated `--module module@version` flags replace file modules.

## Runtime Report Schema

`service_name`:
Runtime service name, for example `billing-service`, `notification_service`, or `platform/api-gateway`.

`environment`:
Deployment environment, for example `production`, `staging`, or `eu-west/prod`.

`git_commit`:
Source commit deployed by the service. In GitLab CI this usually comes from `CI_COMMIT_SHA`.

`build_version`:
Build identifier, release tag, image tag, or deploy version. In GitLab CI this can come from `PROTORADAR_BUILD_VERSION`, `CI_COMMIT_TAG`, or `CI_COMMIT_SHORT_SHA`.

`modules`:
List of reported protobuf module usages. Each entry contains `module` and `version`.

## Drift Statuses

ProtoRadar calculates drift for every reported module usage:

- `unknown_version`: module or version was not found in ProtoRadar.
- `deprecated_version`: reported module version exists and is marked deprecated.
- `behind_latest`: reported version is known, is not deprecated, and is older than the latest known version.
- `up_to_date`: reported version is known, is not deprecated, and is the latest known version.

Drift precedence is `unknown_version`, then `deprecated_version`, then `behind_latest`, then `up_to_date`. Deprecated versions take precedence over `behind_latest`; if the latest known version is deprecated and a service reports it, the usage is still `deprecated_version`. Latest version calculation remains the latest published version and does not exclude deprecated versions.

Drift is not a report failure. Runtime reports succeed even when usages are behind latest or unknown, because reporting stale or unknown data is still useful inventory.

Example: if `billing-service` reports `user-api@v1.0.0` and `user-api@v1.0.0` is marked deprecated, the accepted report includes a usage similar to:

```json
{
  "module": "user-api",
  "version": "v1.0.0",
  "latest_version": "v1.2.0",
  "drift_status": "deprecated_version",
  "drift_reason": "deprecated_version"
}
```

If the reported version is not found in the registry, ProtoRadar reports `unknown_version` instead. Unknown versions have no stored module-version metadata, so they cannot be classified as deprecated.

## Runtime Impact For Breaking Changes

Breaking report runtime impact identifies services that are currently using the exact base module version from a breaking report. This is an MVP exact-version match, not SemVer range analysis.

Runtime impact uses a separate status, `potentially_affected_by_breaking_change`, in breaking-report context. It is not a normal runtime drift status and does not replace `deprecated_version`, `behind_latest`, `unknown_version`, or `up_to_date` in runtime inventory reports.

Runtime impact appears in:

- the breaking report details page in the Web UI;
- GitLab MR bot comments under the Runtime impact section.

If no runtime services are known to use the affected module version, ProtoRadar shows an empty-state message instead of failing the breaking check.

## API Usage

Runtime report:

```http
POST /api/v1/runtime/reports
```

Runtime queries:

```http
GET /api/v1/runtime/services
GET /api/v1/runtime/services/{service}
GET /api/v1/runtime/environments/{environment}
GET /api/v1/modules/{module}/runtime-usages
GET /api/v1/breaking-reports/{report_id}/runtime-impact
```

All endpoints use existing Bearer token authentication. Do not include API tokens in request bodies, logs, CI artifacts, or Markdown comments.

## CLI Usage

Flag-based report:

```sh
protoradar runtime report \
  --service billing-service \
  --environment production \
  --git-commit "$CI_COMMIT_SHA" \
  --build-version "$CI_COMMIT_TAG" \
  --module user-api@v1.2.0 \
  --module billing-api@v1.4.0
```

File-based report:

```sh
protoradar runtime report --from-file protoradar-runtime.yaml
```

Supported flags:

- `--service`
- `--environment`
- `--git-commit`
- `--build-version`
- `--module module@version`, repeatable
- `--from-file path`

GitLab CI defaults:

- `--service`: `PROTORADAR_SERVICE` or `CI_PROJECT_NAME`
- `--environment`: `PROTORADAR_ENVIRONMENT` or `CI_ENVIRONMENT_NAME`
- `--git-commit`: `CI_COMMIT_SHA`
- `--build-version`: `PROTORADAR_BUILD_VERSION`, `CI_COMMIT_TAG`, or `CI_COMMIT_SHORT_SHA`

Exit codes:

- `0`: report accepted, including drift results such as `behind_latest` or `unknown_version`.
- `2`: invalid input, auth, network, server, config, or internal error.

## Web UI

Runtime inventory pages:

- `/ui/runtime/services`: runtime services list with environment and drift summary.
- `/ui/runtime/services/{service}`: service deployments and module usages.
- `/ui/runtime/environments/{environment}`: services currently reported in an environment.
- `/ui/modules/{module}/runtime-usages`: services using a module.
- `/ui/breaking-reports/{report_id}`: includes a Runtime impact section.

The Web UI is read-only and does not expose tokens.

## GitLab MR Comments

The GitLab MR bot comment includes Runtime impact after the Potentially Affected Modules section. When runtime impact exists, the comment shows service, environment, used module version, build version, and commit. If lookup fails, the breaking-check result remains authoritative and the comment renders a warning telling users to check CI logs.

## GitLab CI Template

Use `examples/gitlab/protoradar-runtime-report.yml` after deployment jobs. The template uses CLI environment auth and supports:

- `PROTORADAR_SERVICE`
- `PROTORADAR_ENVIRONMENT`
- `PROTORADAR_BUILD_VERSION`
- `PROTORADAR_RUNTIME_FILE`
- `PROTORADAR_CLI_IMAGE`
- `PROTORADAR_MODULE`
- GitLab predefined variables such as `CI_PROJECT_NAME`, `CI_ENVIRONMENT_NAME`, `CI_COMMIT_SHA`, `CI_COMMIT_TAG`, and `CI_COMMIT_SHORT_SHA`.

## Limitations

- No runtime SDKs yet.
- No Kubernetes operator yet.
- Reports are deployment snapshots, not continuous heartbeats.
- Stale report detection is basic or not implemented yet.
- Runtime impact uses exact base version matching in the MVP.
- No generated client usage detection yet.
- No token scopes or RBAC unless configured externally around ProtoRadar.
- No automatic Slack/email/runtime alerting in Community.
- No un-deprecate workflow is exposed by the Community API or CLI yet.
