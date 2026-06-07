# ProtoRadar

ProtoRadar is a self-hosted protobuf governance platform for small teams. It gives teams a practical place to publish protobuf modules, store Buf-compatible artifacts, run breaking-change checks, wire checks into GitLab, inspect dependency impact, and report which protobuf versions are running in production.

ProtoRadar exists because many teams need protobuf governance without adopting a large managed platform. Community v1.0 focuses on a simple self-hosted workflow: Docker Compose, PostgreSQL, S3-compatible object storage, a CLI, GitLab CI templates, a read-only Web UI, ownership, approval governance, Prometheus metrics, and structured logs.

## Features

- Self-hosted protobuf registry for modules, versions, artifacts, and descriptor metadata.
- Buf-compatible publishing with server-side `buf build`, optional `buf lint`, source archive storage, and Buf image storage.
- Server-side breaking change checks against published baselines.
- Governance layer with module owners, built-in approval policies, approval requests, approval decisions, and audit trail.
- Open-core-friendly Community Edition boundary with explicit identity, authorization, audit, approval workflow, capability, and edition extension points.
- GitLab CI templates for breaking checks, tag publishing, MR bot checks, and runtime inventory reporting.
- GitLab MR bot command for merge request comments, commit status, affected modules, runtime impact context, and governance status.
- Read-only Basic Web UI for local/demo inspection, including governance status.
- Direct dependency graph for imports, field type references, and RPC method references.
- Runtime inventory reporting for deployed service/module versions and drift status.
- Runtime impact lookup for breaking reports.
- Prometheus metrics at `/metrics`.
- Structured request logs with request IDs.
- Docker Compose quickstart with PostgreSQL and MinIO.
- Transport-neutral transactional outbox lifecycle for state-changing workflows.

## Quickstart

Start the local stack:

```sh
cp .env.example .env
docker compose up --build
```

Server config is loaded from built-in defaults, then an optional YAML file passed with `protoradar-server --config <path>`, then environment variables. Environment variables use `PROTORADAR_<SECTION>_<FIELD>` names such as `PROTORADAR_SERVER_HTTP_ADDR` and override YAML values. See [Configuration](docs/configuration.md).

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
bin/protoradar edition
```

Run a breaking check:

```sh
bin/protoradar check-breaking user-api --path examples/repos/user-api --against v1.0.0
```

Add a module owner and request approval for a breaking report:

```sh
bin/protoradar module owners add user-api \
  --subject-type team \
  --subject platform-team \
  --role owner

bin/protoradar approvals request --report-id <breaking_report_id>
```

Report runtime inventory:

```sh
bin/protoradar runtime report --from-file examples/repos/billing-api/protoradar-runtime.yaml
```

## CLI Container Image

The Dockerfile includes a `cli` target for GitLab CI and other non-interactive automation. The image contains the `protoradar` CLI only; it talks to a running ProtoRadar server through the REST API using `PROTORADAR_SERVER_URL` and `PROTORADAR_TOKEN`.

Build and smoke-test a local CLI image:

```sh
make docker-build-cli CLI_IMAGE=protoradar-cli:local
make docker-smoke-cli CLI_IMAGE=protoradar-cli:local
```

Equivalent direct Docker commands:

```sh
docker build --target cli -t protoradar-cli:local .
docker run --rm protoradar-cli:local version
```

This repository does not define a public registry publishing workflow for the CLI image. For GitLab shared runners, build and push the image to your registry, then set `PROTORADAR_CLI_IMAGE`, for example `registry.example.com/platform/protoradar-cli:v1.0.0`.

## Documentation

- [Quickstart](docs/quickstart.md)
- [Architecture](docs/architecture.md)
- [Configuration](docs/configuration.md)
- [CLI](docs/cli.md)
- [REST API](docs/api.md)
- [Buf Workflow](docs/buf-workflow.md)
- [Breaking Checks](docs/breaking-checks.md)
- [Governance](docs/governance.md)
- [Enterprise Boundary](docs/enterprise-boundary.md)
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
- Community Edition implementations are wired through explicit extension points for identity, authorization, audit, approval workflow, capabilities, and edition metadata.

See [Architecture](docs/architecture.md) for details.

## Roadmap And Known Limitations

Community v1.0 intentionally focuses on a polished small-team self-hosted workflow. Known limitations:

- Dependency graph is direct-only; no transitive traversal yet.
- Runtime reports are deployment snapshots, not continuous heartbeats or service discovery.
- Basic Web UI has no built-in login/RBAC; use a trusted network or authentication proxy.
- Governance uses Community identity/authorization/workflow implementations; OIDC, LDAP, advanced RBAC, GitLab group sync, license management, and custom policy DSL are not implemented in OSS.
- Waiver workflows are not implemented.
- Kubernetes operator workflows are not implemented.
- Breaking diagnostic parsing is best-effort and depends on Buf output.
- Generated SDK workflows and client usage tracking are not implemented.
- Metrics are always exposed; there is no separate metrics config flag in Community v1.0.
- Pre-v1.0 development schemas may require fresh install or manual migration.

See [Upgrading](docs/upgrading.md) and [Changelog](CHANGELOG.md).

## Security Notes

- Do not commit API tokens, bootstrap tokens, S3 credentials, or GitLab tokens.
- Store CI tokens as masked variables.
- The API returns raw API tokens only once; PostgreSQL stores token hashes.
- Normal authenticated API routes use API-token authentication and pass through the Authorizer boundary; health, readiness, metrics, and bootstrap token creation are the intended exceptions.
- Request logs do not include Authorization headers or raw tokens.
- Do not expose the Web UI publicly without an authentication proxy.

## License

ProtoRadar is released under the [MIT License](LICENSE).
