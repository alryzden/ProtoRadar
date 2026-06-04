# Basic Web UI

## Overview

ProtoRadar includes a Basic Web UI for local demos and internal inspection. It is a read-only interface for exploring:

- modules;
- versions;
- stored source and Buf image artifacts;
- protobuf descriptor metadata;
- breaking reports;
- direct dependency graph data.

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
  --path examples/user-api
```

Inspect the UI:

- open `/ui/modules` to see the module list;
- open `/ui/modules/user-api` to see module details and versions;
- open `/ui/modules/user-api/versions/v1.0.0` to inspect artifacts, Buf config, and descriptor metadata.
- open `/ui/modules/user-api/dependencies` to inspect downstream consumers, upstream dependencies, and unresolved dependencies.

Run a breaking check:

```sh
protoradar check-breaking user-api \
  --path examples/user-api \
  --against latest \
  --target-ref local-demo
```

Then open `/ui/breaking-reports` and the generated report detail page.
Breaking report detail pages include potentially affected modules when downstream consumers are known.

## Pages

- `/ui/modules`: module list with latest version and compatibility summary.
- `/ui/modules/{module}`: module details, versions, and recent breaking reports.
- `/ui/modules/{module}/dependencies`: direct downstream consumers, upstream dependencies, and unresolved dependencies.
- `/ui/modules/{module}/versions/{version}`: version details, artifacts, Buf config, descriptor metadata, and related reports.
- `/ui/breaking-reports`: breaking report list.
- `/ui/breaking-reports/{report_id}`: breaking report summary, change details, and potentially affected modules.

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

## Security Notes

- The UI is read-only.
- No login or RBAC exists yet.
- Do not expose the UI publicly without authentication or a trusted reverse proxy.
- No tokens or secrets are displayed by the UI.
- The UI should be treated as local/demo/internal-only until authentication and authorization are added.

## Limitations

- No login/RBAC yet.
- Dependency graph support is direct-only; transitive traversal is not implemented yet.
- No runtime inventory UI yet.
- No approval workflow UI yet.
- No write operations from the UI yet.
