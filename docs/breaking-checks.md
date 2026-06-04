# Breaking Change Checks

Phase 3 adds server-side breaking-change checks for Buf-compatible protobuf workspaces. Developers and CI pipelines can compare a proposed workspace against a previously published module version and receive a structured report plus a human-readable summary.

Breaking changes are normal check results. A check that finds breaking changes returns `status=breaking`; it is not an HTTP/server failure.

## Requirements

Before running a check:

- the module must already exist;
- the baseline version must already be published;
- the baseline version must have a stored `buf_image` artifact;
- the proposed workspace must contain `buf.yaml` at the workspace root;
- the proposed workspace must contain at least one `.proto` file.

The baseline `buf_image` is produced during `protoradar push` by server-side `buf build`. Older versions without a `buf_image` cannot be used as breaking-check baselines.

## CLI Usage

Compare against the latest published version:

```sh
protoradar check-breaking user-api \
  --path . \
  --against latest
```

Compare against an explicit version:

```sh
protoradar check-breaking user-api \
  --path . \
  --against v1.0.0
```

Attach a target reference such as a branch, commit SHA, merge request, or CI job identifier:

```sh
protoradar check-breaking user-api \
  --path . \
  --against latest \
  --target-ref feature/user-email-change
```

The CLI packages the proposed workspace the same way as `protoradar push`: it includes `buf.yaml`, optional `buf.lock`, and `.proto` files recursively; preserves relative paths; excludes local/build directories; and rejects symlinks or unsafe archive paths.

The CLI does not run `buf breaking` locally. Server-side Buf execution is authoritative.

## Stable Exit Codes

- `0`: check completed and no breaking changes were found.
- `1`: check completed and breaking changes were found.
- `2`: invalid input, auth error, network error, server/tool/config error, or internal failure.

CI jobs should treat exit code `1` as a completed governance result, not as an infrastructure failure.

## GitLab CI Usage

Use `examples/gitlab/protoradar-breaking-check.yml` to run simple pass/fail breaking checks in merge request pipelines:

```yaml
include:
  - project: your-group/protoradar
    ref: main
    file: examples/gitlab/protoradar-breaking-check.yml

stages:
  - validate

variables:
  PROTORADAR_SERVER_URL: "https://protoradar.example.com"
  PROTORADAR_MODULE: "user-api"
  PROTORADAR_PROTO_PATH: "."
  PROTORADAR_AGAINST: "latest"
  PROTORADAR_CLI_IMAGE: "registry.example.com/platform/protoradar-cli:latest"

protoradar:breaking-check:
  extends: .protoradar-breaking-check
  stage: validate
```

The template writes a human-readable report artifact and passes `--target-ref "${CI_COMMIT_SHA:-local}"`. Store `PROTORADAR_TOKEN` as a masked GitLab CI/CD variable; do not commit it.

For GitLab merge request comments and commit statuses, use `examples/gitlab/protoradar-mr-check.yml` and the `protoradar gitlab mr-check` command. See [GitLab CI Integration](gitlab-ci.md) and [GitLab Merge Request Bot](gitlab-mr-bot.md) for the full CI setup.

For local/demo inspection, stored reports can also be viewed in the read-only Basic Web UI at `/ui/breaking-reports`. Report detail pages include potentially affected modules when direct downstream consumers are known. See [Basic Web UI](web-ui.md) and [Dependency Graph MVP](dependency-graph.md).

## Example Workflow

Publish a baseline:

```sh
protoradar push user-api \
  --version v1.0.0 \
  --path examples/user-api
```

Modify `examples/user-api/proto/user/v1/user.proto` in a breaking way, for example change a field type or remove an RPC.

Run a check:

```sh
protoradar check-breaking user-api \
  --path examples/user-api \
  --against latest
```

Example output:

```text
ProtoRadar Breaking Change Report

Module: user-api
Against: v1.0.0
Target: local
Status: breaking
Changes: 1

Breaking changes:
1. proto/user/v1/user.proto
   Rule: FIELD_SAME_TYPE
   Symbol: user.v1.User.display_name
   Message: Field "display_name" changed type from string to bytes.

Result: breaking changes found.
```

Expected exit code: `1`.

## REST API

All breaking-check endpoints require `Authorization: Bearer <token>`.

Create a check:

```sh
curl -sS \
  -H "Authorization: Bearer <token>" \
  -F "against=latest" \
  -F "target_ref=local" \
  -F "artifact=@source.tar.gz" \
  http://localhost:8080/api/v1/modules/user-api/breaking-checks
```

Retrieve a report:

```sh
curl -sS \
  -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/breaking-reports/<report_id>
```

List reports for a module:

```sh
curl -sS \
  -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/modules/user-api/breaking-reports
```

Retrieve direct downstream modules affected by a stored report's module:

```sh
curl -sS \
  -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/breaking-reports/<report_id>/affected-modules
```

Create-check responses use HTTP `200` for both `passed` and `breaking` statuses. Request, auth, baseline, archive, tool, or internal failures use error status codes.

The Basic Web UI exposes the same stored report data:

```text
/ui/breaking-reports
/ui/breaking-reports?module=user-api
/ui/breaking-reports?status=breaking
/ui/breaking-reports/<report_id>
```

## Report Model

Reports include:

- `id`: report identifier;
- `module`: module name;
- `against`: baseline version;
- `target_ref`: caller-supplied target reference or `local`;
- `status`: `passed` or `breaking` for completed checks;
- `change_count`: number of stored changes;
- `changes`: structured breaking-change diagnostics;
- `human_summary`: reusable text report for CLI output;
- `created_at`: report creation time.

Affected-module lookups for reports are based on the current direct dependency graph for the report module. Phase 7 does not include transitive affected-module traversal.

Each change can include:

- `category`;
- `file_path`;
- `package_name`;
- `symbol`;
- `rule_id`;
- `message`;
- `severity`.

The diagnostic parser is conservative and best-effort. Raw Buf output is captured with a size limit, but REST responses expose the structured fields and human summary rather than unbounded tool output.

## How ProtoRadar Uses Buf

For a breaking check, ProtoRadar:

1. Receives a proposed source archive from the CLI or REST API.
2. Extracts it with the same safe archive checks used by publish.
3. Requires `buf.yaml` at the proposed workspace root.
4. Downloads the baseline version's stored `buf_image` artifact.
5. Runs `buf breaking <proposed-workspace> --against <baseline-image>` in the infrastructure Buf adapter.
6. Parses diagnostics best-effort into structured changes.
7. Stores the report and changes.

The concrete Buf CLI execution lives in `internal/infrastructure/bufcli`. Domain and usecase packages depend only on application-level ports.

## Transactional Outbox

`BreakingReportCreated` is written inside the same PostgreSQL transaction as the `breaking_reports` row and its `breaking_changes` rows.

Usecases never publish directly to Kafka, Sarama, or any broker. Transport-specific routing and publishing stay outside usecases. Raw API tokens and uploaded source contents are not included in the event payload.

## Dependency Impact

Breaking report details can show potentially affected modules through the dependency graph read model. The report-specific endpoint is:

```text
GET /api/v1/breaking-reports/{report_id}/affected-modules
```

The GitLab MR bot uses the same affected-module data in merge request comments. If no downstream modules are known, ProtoRadar reports that no downstream modules are currently known to depend on the checked module.

See [Dependency Graph MVP](dependency-graph.md) for dependency detection rules and limitations.

## Configuration

Breaking-check limits are configured separately from publish lint/build report capture:

```text
PROTORADAR_BREAKING_MAX_REPORT_BYTES=32768
PROTORADAR_BREAKING_MAX_CHANGES=1000
PROTORADAR_BREAKING_DEFAULT_AGAINST=latest
```

Equivalent YAML:

```yaml
breaking:
  max_report_bytes: 32768
  max_changes: 1000
  default_against: latest
```

## Known Limitations

- Affected-consumer analysis is direct-only; transitive traversal is planned later.
- No approval or waiver workflow yet.
- Diagnostic parsing is best-effort and may not extract every Buf diagnostic field perfectly.
- The Web UI is read-only and does not include report approval.
