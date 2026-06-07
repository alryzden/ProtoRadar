# Basic Web UI

## Overview

ProtoRadar includes a Basic Web UI for local demos and internal inspection. It is a read-only interface for exploring:

- modules;
- versions;
- stored source and Buf image artifacts;
- protobuf descriptor metadata;
- breaking reports;
- direct dependency graph data;
- runtime inventory and drift;
- governance owners, approval status, approval request details, and audit trail;
- Community edition, version/build metadata, and capability status.

The UI is intended to make registry state easier to inspect without relying only on CLI output or raw REST responses.

## Enabling the UI

The UI is controlled by server config:

- `ui.enabled`: enables or disables UI route registration. Default: `true`.
- `ui.base_path`: base route for UI pages. Default: `/ui`.
- `ui.static_path`: route for UI static assets. Default: `/ui/static`.

Equivalent environment variables:

```text
PROTORADAR_UI_ENABLED=true
PROTORADAR_UI_BASE_PATH=/ui
PROTORADAR_UI_STATIC_PATH=/ui/static
```

Example YAML config:

```yaml
ui:
  enabled: true
  base_path: /ui
  static_path: /ui/static
```

`ui.base_path` and `ui.static_path` must start with `/`, and `ui.static_path` must stay under `ui.base_path`.

## Local Demo

Start the local stack:

```sh
docker compose up
```

Open:

```text
http://localhost:8080/ui
```

The `/ui` route redirects to `/ui/modules`.

## Demo Workflow

Create an API token with the local bootstrap token, then log in with the CLI:

```sh
curl -sS \
  -H "Authorization: Bearer local-bootstrap-token" \
  -H "Content-Type: application/json" \
  -d '{"name":"local-cli"}' \
  http://localhost:8080/api/v1/tokens

protoradar login \
  --server http://localhost:8080 \
  --token <token>
```

Create a module:

```sh
protoradar module create user-api \
  --description "User service protobuf contracts" \
  --repository-url "https://gitlab.example.com/platform/user-api"
```

Push a version:

```sh
protoradar push user-api \
  --version v1.0.0 \
  --path examples/repos/user-api
```

Inspect the UI:

- open `/ui/modules` to see the module list;
- open `/ui/modules/user-api` to see module details and versions;
- open `/ui/modules/user-api` to see configured owners and maintainers;
- open `/ui/modules/user-api/versions/v1.0.0` to inspect artifacts, Buf config, and descriptor metadata.
- open `/ui/modules/user-api/dependencies` to inspect downstream consumers, upstream dependencies, and unresolved dependencies.
- open `/ui/modules/user-api/runtime-usages` to inspect services reporting runtime usage of the module.

Report runtime inventory:

```sh
protoradar runtime report \
  --service billing-service \
  --environment production \
  --git-commit abc1234 \
  --build-version 2026.06.04-15 \
  --module user-api@v1.0.0
```

Then open `/ui/runtime/services` to inspect runtime service summaries and `/ui/runtime/environments/production` to inspect the environment inventory.

Run a breaking check:

```sh
protoradar check-breaking user-api \
  --path examples/repos/user-api \
  --against latest \
  --target-ref local-demo
```

Then open `/ui/breaking-reports` and the generated report detail page.
Breaking report detail pages include potentially affected modules when downstream consumers are known and runtime impact when services report usage of the affected module version.

Create governance state:

```sh
protoradar module owners add user-api \
  --subject-type team \
  --subject platform-team \
  --role owner

protoradar approvals request \
  --report-id <breaking_report_id>
```

Then open the breaking report detail page to inspect approval status and follow the approval request link for requirements, decisions, and audit trail.

## Pages

- `/ui/modules`: module list with latest version and compatibility summary.
- `/ui/modules/{module}`: module details, owners/maintainers, versions, and recent breaking reports.
- `/ui/modules/{module}/dependencies`: direct downstream consumers, upstream dependencies, and unresolved dependencies.
- `/ui/modules/{module}/runtime-usages`: services and environments reporting runtime usage of a module.
- `/ui/modules/{module}/versions/{version}`: version details, artifacts, Buf config, descriptor metadata, and related reports.
- `/ui/runtime/services`: runtime services with environments, last report time, and drift summary.
- `/ui/runtime/services/{service}`: runtime service deployments and module usages.
- `/ui/runtime/environments/{environment}`: services and module usages reported in an environment.
- `/ui/breaking-reports`: breaking report list.
- `/ui/breaking-reports/{report_id}`: breaking report summary, change details, potentially affected modules, runtime impact, and approval status when an approval request exists.
- `/ui/approval-requests/{request_id}`: approval request status, requirements, decisions, and governance audit trail.
- `/ui/about`: Community edition, version/build metadata, enabled capabilities, and unavailable enterprise capabilities.

Runtime pages display server-calculated drift statuses, including `deprecated_version`, as badges. Breaking report runtime impact shows both the breaking impact status and the stored runtime drift status for matching usages. The Web UI does not calculate drift itself and does not implement enterprise runtime alerts.

Deprecated runtime usage appears anywhere runtime module usages are shown:

- service details;
- environment inventory;
- module runtime usages;
- runtime service summaries through the Deprecated count;
- breaking report runtime impact when the matched usage is also deprecated.

The UI renders the server-returned drift status and reason only. It does not look up module version deprecation metadata independently and does not expose API tokens. Deprecation reason/date/actor are available from module version API responses when the server returns them for that version.

Static CSS is served from `/ui/static/app.css` by default.

## Search and Filtering

Module search:

```text
/ui/modules?q=user
```

Breaking report filters:

```text
/ui/breaking-reports?module=user-api
/ui/breaking-reports?status=breaking
/ui/breaking-reports?q=feature
```

Filters can be combined:

```text
/ui/breaking-reports?module=user-api&status=breaking&q=feature
```

Runtime service filters:

```text
/ui/runtime/services?q=billing
/ui/runtime/services?environment=production
/ui/runtime/services?drift_status=behind_latest
```

## Security Notes

- The UI is read-only, including governance pages.
- No login or RBAC exists yet.
- Do not expose the UI publicly without authentication or a trusted reverse proxy.
- No tokens or secrets are displayed by the UI.
- `/ui/about` reports capabilities and build metadata only; it does not display tokens, hashes, credentials, or license data.
- The UI should be treated as local/demo/internal-only until authentication and authorization are added.

## Limitations

- No login/RBAC yet.
- Dependency graph support is direct-only; transitive traversal is not implemented yet.
- Runtime inventory reports are snapshots, not continuous heartbeats.
- Governance approval state is visible, but approve/reject actions are not available from the UI.
- No write operations from the UI yet.
- The UI can display deprecated runtime drift, but it cannot mark or unmark versions as deprecated.
- No automatic Slack/email/runtime alerting is implemented in Community.
