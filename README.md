# ProtoRadar

ProtoRadar is a self-hosted protobuf governance platform for small teams. It gives teams a practical place to publish protobuf modules, store Buf-compatible artifacts, run breaking-change checks, wire checks into GitLab, inspect dependency impact, and report which protobuf versions are running in production.

ProtoRadar exists because many teams need protobuf governance without adopting a large managed platform. Community v1.0 focuses on a simple self-hosted workflow: Docker Compose, PostgreSQL, S3-compatible object storage, a CLI, GitLab CI templates, a read-only Web UI, Prometheus metrics, and structured logs.

## Features

- Self-hosted protobuf registry for modules, versions, artifacts, and descriptor metadata.
- Buf-compatible publishing with server-side `buf build`, optional `buf lint`, source archive storage, and Buf image storage.
- Server-side breaking change checks against published baselines.
- GitLab CI templates for breaking checks, tag publishing, MR bot checks, and runtime inventory reporting.
- GitLab MR bot command for merge request comments, commit status, affected modules, and runtime impact context.
- Read-only Basic Web UI for local/demo inspection.
- Direct dependency graph for imports, field type references, and RPC method references.
- Runtime inventory reporting for deployed service/module versions and drift status.
- Runtime impact lookup for breaking reports.
- Prometheus metrics at `/metrics`.
- Structured request logs with request IDs.
- Docker Compose quickstart with PostgreSQL and MinIO.
- Transactional outbox records for state-changing workflows.

## Quickstart

Start the local stack:

```sh
cp .env.example .env
docker compose up --build
```

In another terminal:

```sh
make build
make smoke-test
make demo
```

Open:

```text
http://localhost:8080/ui
```

Local services:

- ProtoRadar API: `http://localhost:8080`
- Web UI: `http://localhost:8080/ui`
- Metrics: `http://localhost:8080/metrics`
- MinIO console: `http://localhost:9001`
- PostgreSQL: `localhost:5432`

For the full walkthrough, see [Quickstart](docs/quickstart.md).

## Basic Usage

Create an API token with the local bootstrap token:

```sh
curl -sS \
  -H "Authorization: Bearer local-bootstrap-token" \
  -H "Content-Type: application/json" \
  -d '{"name":"local-cli"}' \
  http://localhost:8080/api/v1/tokens
```

Login or use environment auth:

```sh
bin/protoradar login --server http://localhost:8080 --token <token>

export PROTORADAR_SERVER_URL=http://localhost:8080
export PROTORADAR_TOKEN=<token>
```

Publish examples:

```sh
bin/protoradar module create user-api --description "User service protobuf contracts"
bin/protoradar push user-api --version v1.0.0 --path examples/repos/user-api

bin/protoradar module create billing-api --description "Billing service protobuf contracts"
bin/protoradar push billing-api --version v1.0.0 --path examples/repos/billing-api

bin/protoradar module dependencies user-api
bin/protoradar module affected user-api
```

Run a breaking check:

```sh
bin/protoradar check-breaking user-api --path examples/repos/user-api --against v1.0.0
```

Report runtime inventory:

```sh
bin/protoradar runtime report --from-file examples/repos/billing-api/protoradar-runtime.yaml
```

## Documentation

- [Quickstart](docs/quickstart.md)
- [Architecture](docs/architecture.md)
- [Configuration](docs/configuration.md)
- [CLI](docs/cli.md)
- [REST API](docs/api.md)
- [Buf Workflow](docs/buf-workflow.md)
- [Breaking Checks](docs/breaking-checks.md)
- [GitLab CI](docs/gitlab-ci.md)
- [GitLab MR Bot](docs/gitlab-mr-bot.md)
- [Dependency Graph](docs/dependency-graph.md)
- [Runtime Inventory](docs/runtime-inventory.md)
- [Web UI](docs/web-ui.md)
- [Deployment](docs/deployment.md)
- [Development](docs/development.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Upgrading](docs/upgrading.md)
- [Examples](examples/README.md)
- [Changelog](CHANGELOG.md)

## Architecture Principles

ProtoRadar follows clean architecture with explicit package boundaries:

- domain contains business vocabulary and ports only;
- usecases orchestrate business workflows and transactions;
- PostgreSQL, S3/MinIO, Buf CLI, and GitLab API live behind adapters;
- state-changing usecases write transactional outbox records inside the same DB transaction as business state;
- transports and CLI use application/usecase interfaces or REST clients.

See [Architecture](docs/architecture.md) for details.

## Roadmap And Known Limitations

Community v1.0 intentionally focuses on a polished small-team self-hosted workflow. Known limitations:

- Dependency graph is direct-only; no transitive traversal yet.
- Runtime reports are deployment snapshots, not continuous heartbeats or service discovery.
- Basic Web UI has no built-in login/RBAC; use a trusted network or authentication proxy.
- Approval, waiver, and policy workflow features are not implemented.
- OAuth/OIDC and Kubernetes operator workflows are not implemented.
- Breaking diagnostic parsing is best-effort and depends on Buf output.
- Generated SDK workflows and client usage tracking are not implemented.
- Metrics are always exposed; there is no separate metrics config flag in Community v1.0.
- Pre-v1.0 development schemas may require fresh install or manual migration.

See [Upgrading](docs/upgrading.md) and [Changelog](CHANGELOG.md).

## Security Notes

- Do not commit API tokens, bootstrap tokens, S3 credentials, or GitLab tokens.
- Store CI tokens as masked variables.
- The API returns raw API tokens only once; PostgreSQL stores token hashes.
- Request logs do not include Authorization headers or raw tokens.
- Do not expose the Web UI publicly without an authentication proxy.

## License

ProtoRadar is released under the [MIT License](LICENSE).
