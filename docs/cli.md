# CLI Reference

The `protoradar` CLI talks to the ProtoRadar server over REST. It does not access PostgreSQL or object storage directly.

## Authentication

Login writes local credentials to `~/.config/protoradar/config.yaml` with `0600` permissions:

```sh
protoradar login --server http://localhost:8080 --token <token>
```

CI usage can avoid local config:

```sh
export PROTORADAR_SERVER_URL=https://protoradar.example.com
export PROTORADAR_TOKEN=<token>
```

Do not print or commit token values.

## HTTP Timeout

CLI-created ProtoRadar and GitLab HTTP clients use a finite `30s` timeout by default. Override it with:

```sh
export PROTORADAR_CLI_HTTP_TIMEOUT=45s
```

Local CLI config also supports:

```yaml
http_timeout: 45s
```

The timeout must be a positive Go duration string such as `30s`, `1m`, or `2m30s`. Injected test/custom HTTP clients keep their configured timeout.

## CLI Container Image

The Dockerfile has a `cli` target that packages the `protoradar` CLI for CI jobs. The image does not contain the server runtime or database; it calls the ProtoRadar server over REST using `PROTORADAR_SERVER_URL` and `PROTORADAR_TOKEN`.

Build locally:

```sh
make docker-build-cli CLI_IMAGE=protoradar-cli:local
```

Or with Docker directly:

```sh
docker build --target cli -t protoradar-cli:local .
```

Smoke test:

```sh
make docker-smoke-cli CLI_IMAGE=protoradar-cli:local
docker run --rm protoradar-cli:local version
```

GitLab CI templates use this image through `PROTORADAR_CLI_IMAGE`. This repository does not publish an official public CLI image; build and push the image to a registry your runners can access, then set `PROTORADAR_CLI_IMAGE` to that tag.

## Exit Codes

- `0`: success, safe result, or no breaking changes.
- `1`: breaking changes found by `check-breaking` or `gitlab mr-check`.
- `2`: invalid input, auth, network, server, config, tool, or internal error.

Runtime drift statuses are successful results and exit `0`.

Approval request creation is idempotent and exits `0` when it returns an existing request. Approval rejection commands are successful governance decisions and exit `0`. Repeated or conflicting decisions, rejected actor overrides, auth failures, network failures, and server errors exit `2`.

## Commands

### `protoradar version`

Print build metadata:

```sh
protoradar version
```

Output includes version, commit, and build date.

### `protoradar edition`

Fetch server edition and capabilities:

```sh
protoradar edition
```

Example output:

```text
Edition: community
Version: v1.2.0

Enabled capabilities:
- registry
- breaking_checks
- gitlab_mr_bot
- dependency_graph
- runtime_inventory
- community_governance

Unavailable enterprise capabilities:
- oidc_auth
- advanced_rbac
- gitlab_group_sync
```

Auth, network, server, config, and internal errors exit `2`.

### `protoradar module create`

```sh
protoradar module create user-api \
  --description "User service protobuf contracts" \
  --repository-url "https://gitlab.example.com/platform/user-api"
```

### `protoradar module list` / `protoradar list`

```sh
protoradar module list
protoradar list
```

Both list modules. `protoradar list` is the short form.

### `protoradar module link-gitlab`

```sh
protoradar module link-gitlab user-api \
  --project-id 12345 \
  --project-path platform/user-api \
  --gitlab-base-url https://gitlab.example.com
```

If flags are omitted in GitLab CI, the CLI can use `CI_PROJECT_ID`, `CI_PROJECT_PATH`, and `CI_SERVER_URL`.

### `protoradar push`

Publish a Buf-compatible workspace:

```sh
protoradar push user-api \
  --version v1.0.0 \
  --path examples/repos/user-api
```

The path must contain `buf.yaml` and at least one `.proto` file. The CLI uploads a source archive. The server runs Buf build/lint, stores source and Buf image artifacts, extracts descriptor metadata, and updates dependency graph data.

### `protoradar pull`

Download and extract the source archive for a published version:

```sh
protoradar pull user-api \
  --version v1.0.0 \
  --output ./tmp/user-api
```

Use `--force` to overwrite a non-empty output directory when supported by the command help.

### `protoradar module version deprecate`

Mark a published module version as deprecated:

```sh
protoradar module version deprecate user-api v1.0.0 \
  --reason "Use v1.2.0 instead."
```

The command calls `POST /api/v1/modules/{module}/versions/{version}/deprecate`. It does not accept `--actor`; the server records `deprecated_by` from the authenticated principal associated with `PROTORADAR_TOKEN` or local login credentials.

Example output:

```text
Deprecated user-api v1.0.0
Deprecated at: 2026-06-04T12:30:00Z
Deprecated by: api-token:release-bot
Reason: Use v1.2.0 instead.
```

Deprecating a version is metadata-only. It does not delete artifacts and does not make `protoradar pull` fail by itself. Re-running the command for an already deprecated version returns the existing deprecation metadata and exits `0`.

### `protoradar check-breaking`

```sh
protoradar check-breaking user-api \
  --path examples/repos/user-api \
  --against latest \
  --target-ref feature/user-api \
  --report-file protoradar-breaking-report.txt
```

Exit `1` means breaking changes were found. It is not a server/tool failure.

### `protoradar gitlab mr-check`

Run a breaking check and update a GitLab merge request comment/status:

```sh
protoradar gitlab mr-check \
  --module user-api \
  --path . \
  --against latest \
  --governance=true \
  --report-file protoradar-mr-report.md
```

In GitLab CI, the command uses predefined CI variables for project, merge request, commit, and server URL. It reads the GitLab API token from `PROTORADAR_GITLAB_TOKEN` or `GITLAB_TOKEN`; prefer environment variables over `--gitlab-token` so tokens do not appear in command arguments. See [GitLab MR Bot](gitlab-mr-bot.md).

Use `--governance=true` to allow approved breaking changes to pass the MR check. Without it, breaking changes keep the pre-governance exit behavior and exit `1`.

Governance mode creates approval requests through the ProtoRadar REST API without sending an actor by default. The server records the authenticated `PROTORADAR_TOKEN` principal as the effective governance actor. The deprecated `--governance-actor` flag is compatibility-only; use it only when the server explicitly enables actor override.

### `protoradar module dependencies`

Show upstream dependencies, downstream consumers, and unresolved dependencies:

```sh
protoradar module dependencies user-api
```

### `protoradar module affected`

Show direct downstream modules affected by a provider module:

```sh
protoradar module affected user-api
```

### `protoradar module owners`

List module owners and maintainers:

```sh
protoradar module owners list user-api
```

Add an owner or maintainer:

```sh
protoradar module owners add user-api \
  --subject-type team \
  --subject platform-team \
  --role owner
```

Remove an owner or maintainer:

```sh
protoradar module owners remove user-api \
  --owner-id <owner_id>
```

`subject-type` must be `user` or `team`. `role` must be `owner` or `maintainer`.
Invalid owner `--subject-type` or `--role` values fail client-side before any API request is sent and exit `2`.

Owner add/remove commands do not require `--actor` by default. The CLI omits actor from the REST request unless you explicitly pass `--actor`. If you do pass it, the server accepts it only when `governance.actor_override_enabled=true`; otherwise the CLI surfaces the server rejection and exits `2`.

### `protoradar approvals`

Create or return an approval request for a breaking report. If the request already exists, the CLI prints the existing approval request/status and exits `0`:

```sh
protoradar approvals request \
  --report-id <breaking_report_id>
```

Show approval status:

```sh
protoradar approvals status \
  --report-id <breaking_report_id>
```

Approve a requirement:

```sh
protoradar approvals approve <requirement_id> \
  --request-id <approval_request_id> \
  --comment "Consumers have been notified."
```

Reject a requirement:

```sh
protoradar approvals reject <requirement_id> \
  --request-id <approval_request_id> \
  --comment "billing-api still uses this version in production."
```

For Phase 10 MVP, approval and rejection require explicit `--request-id`; the CLI does not infer the request from only a requirement ID. A requirement decision is final: the first approve or reject exits `0`, while repeated approve/reject, approve-after-reject, and reject-after-approve surface the server conflict and exit `2`.

Governance commands use the authenticated API-token principal as actor by default. `--actor` remains accepted for migration compatibility, but the server rejects a different actor unless `governance.actor_override_enabled=true`.

Approval request creation omits actor by default. Approval commands do not accept an arbitrary decision value. `approvals approve` always sends `approved`, and `approvals reject` always sends `rejected`. Successful approval and rejection output displays `decided_by` from the server response; if the API does not return an actor value for a different governance response, the CLI does not guess one locally.

Raw API tokens are never printed and are never used as actor strings.

### `protoradar runtime report`

Report deployed module versions with flags:

```sh
protoradar runtime report \
  --service billing-service \
  --environment production \
  --git-commit abc1234 \
  --build-version demo-v1 \
  --module user-api@v1.0.0 \
  --module billing-api@v1.0.0
```

Or report from a YAML/JSON file:

```sh
protoradar runtime report --from-file examples/repos/billing-api/protoradar-runtime.yaml
```

With GitLab CI, defaults can come from `CI_PROJECT_NAME`, `CI_ENVIRONMENT_NAME`, `CI_COMMIT_SHA`, `CI_COMMIT_TAG`, and `CI_COMMIT_SHORT_SHA`.

Runtime report output uses server-calculated drift. If a service reports a known deprecated module version, the usage is shown as `deprecated_version`; unknown versions remain `unknown_version` and are not treated as deprecated.
