# Quickstart

This guide starts a local ProtoRadar stack with Docker Compose, publishes the example protobuf modules, runs a breaking check, reports runtime inventory, and opens the Web UI.

## Prerequisites

- Docker with Compose v2.
- Go 1.26 if you want to build the local CLI with `make build`.
- `curl` for smoke checks.
- Optional: `buf` for local example workspace validation.

## Start The Local Stack

```sh
cp .env.example .env
docker compose up --build
```

In another terminal, check readiness:

```sh
curl -fsS http://localhost:8080/healthz
curl -fsS http://localhost:8080/readyz
```

The local stack exposes:

- ProtoRadar API and Web UI: `http://localhost:8080`
- Web UI: `http://localhost:8080/ui`
- Prometheus metrics: `http://localhost:8080/metrics`
- MinIO API: `http://localhost:9000`
- MinIO console: `http://localhost:9001`
- PostgreSQL: `localhost:5432`

## Build The CLI

```sh
make build
```

This creates:

- `bin/protoradar`
- `bin/protoradar-server`

The Docker image also contains both binaries.

## Create A Local API Token

The local bootstrap token is in `.env` as `PROTORADAR_AUTH_BOOTSTRAP_TOKEN`. The default is `local-bootstrap-token`.

```sh
curl -sS \
  -H "Authorization: Bearer local-bootstrap-token" \
  -H "Content-Type: application/json" \
  -d '{"name":"local-cli"}' \
  http://localhost:8080/api/v1/tokens
```

Use the returned `token` value. Do not commit token values.

Login locally:

```sh
bin/protoradar login \
  --server http://localhost:8080 \
  --token <token>
```

Or use environment authentication without writing a config file:

```sh
export PROTORADAR_SERVER_URL=http://localhost:8080
export PROTORADAR_TOKEN=<token>
```

## Publish Example Modules

Create modules:

```sh
bin/protoradar module create user-api --description "User service protobuf contracts"
bin/protoradar module create billing-api --description "Billing service protobuf contracts"
bin/protoradar module create notification-api --description "Notification service protobuf contracts"
```

Publish the provider first:

```sh
bin/protoradar push user-api --version v1.0.0 --path examples/repos/user-api
```

Then publish consumers:

```sh
bin/protoradar push billing-api --version v1.0.0 --path examples/repos/billing-api
bin/protoradar push notification-api --version v1.0.0 --path examples/repos/notification-api
```

## Inspect Dependencies

```sh
bin/protoradar module dependencies user-api
bin/protoradar module affected user-api
```

`billing-api` and `notification-api` reference `user.v1.User`, so they should appear as downstream consumers of `user-api` after publish.

## Run A Breaking Check

```sh
bin/protoradar check-breaking user-api \
  --path examples/repos/user-api \
  --against v1.0.0
```

Exit codes:

- `0`: no breaking changes.
- `1`: breaking changes found.
- `2`: input, auth, network, server, tool, config, or internal error.

## Report Runtime Inventory

```sh
bin/protoradar runtime report --from-file examples/repos/billing-api/protoradar-runtime.yaml
bin/protoradar runtime report --from-file examples/repos/notification-api/protoradar-runtime.yaml
```

Runtime drift statuses such as `behind_latest` and `unknown_version` are successful report results, not API errors.

## Open The Web UI

Open:

```text
http://localhost:8080/ui
```

Useful pages:

- `http://localhost:8080/ui/modules`
- `http://localhost:8080/ui/modules/user-api/dependencies`
- `http://localhost:8080/ui/runtime/services`
- `http://localhost:8080/ui/runtime/environments/production`

The Web UI is read-only and has no login/RBAC in Community v1.0. Do not expose it publicly without an authentication proxy or trusted network boundary.

## View Metrics

```sh
curl -fsS http://localhost:8080/metrics
```

The response is Prometheus text format and includes HTTP, publish, breaking-check, runtime-report, and dependency graph metrics.

## Run The Demo Or Smoke Test

```sh
make smoke-test
make demo
```

`make smoke-test` creates timestamped smoke modules to avoid repeated-run conflicts. `make demo` uses the stable `user-api`, `billing-api`, and `notification-api` example module names and continues when expected resources already exist.

## Stop Or Clean Up

Stop containers but keep volumes:

```sh
docker compose down
```

Remove local data volumes:

```sh
docker compose down -v
```
