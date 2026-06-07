# Deployment

Community v1.0 is designed for small-team self-hosting with Docker Compose, PostgreSQL, and S3-compatible object storage.

## Docker Compose

Local quickstart:

```sh
cp .env.example .env
docker compose up --build
```

Compose starts:

- `protoradar-server`
- PostgreSQL
- MinIO
- MinIO bucket initialization

For a persistent environment, review every value in `.env` and replace local placeholder secrets.

## PostgreSQL

PostgreSQL stores all relational state: modules, versions, artifact metadata, descriptor metadata, reports, dependency graph, runtime inventory, governance ownership/approval/audit records, API token hashes, and outbox records.

Requirements:

- Use persistent storage for the PostgreSQL data directory.
- Back up the database regularly.
- Run only one server instance applying migrations during startup unless you have deployment coordination.

Migrations run through Goose at server startup from the `migrations/` directory packaged in the image. Goose records migration state in `goose_db_version`. The old custom `schema_migrations` table is not supported, queried, or seeded; ProtoRadar is not public yet, so old migration-runner state compatibility is intentionally not required.

## S3 / MinIO

ProtoRadar stores source archives and Buf image artifacts in S3-compatible storage.

Local Compose uses MinIO and creates the bucket through `minio-init`. For production-like deployment:

- create the bucket before starting the server;
- use least-privilege credentials;
- keep `PROTORADAR_STORAGE_S3_USE_PATH_STYLE=true` for MinIO;
- use TLS for remote object storage endpoints.

Back up object storage together with PostgreSQL. PostgreSQL references object keys, so database and bucket backups should be consistent.

## Configuration

See [Configuration](configuration.md) for all server env vars. At minimum, configure:

- `PROTORADAR_DATABASE_URL`
- `PROTORADAR_STORAGE_S3_ENDPOINT`
- `PROTORADAR_STORAGE_S3_BUCKET`
- `PROTORADAR_STORAGE_S3_ACCESS_KEY`
- `PROTORADAR_STORAGE_S3_SECRET_KEY`
- `PROTORADAR_AUTH_TOKEN_HASH_SECRET`
- `PROTORADAR_AUTH_BOOTSTRAP_TOKEN`

Governance is enabled by default. Review these optional values for production-like deployments:

- `PROTORADAR_GOVERNANCE_ENABLED`
- `PROTORADAR_GOVERNANCE_PRODUCTION_ENVIRONMENTS`
- `PROTORADAR_GOVERNANCE_ALLOW_MAINTAINER_APPROVAL`

HTTP lifecycle timeouts have safe defaults and can be tuned for your proxy/load-balancer behavior:

- `PROTORADAR_SERVER_HTTP_READ_HEADER_TIMEOUT`
- `PROTORADAR_SERVER_HTTP_READ_TIMEOUT`
- `PROTORADAR_SERVER_HTTP_WRITE_TIMEOUT`
- `PROTORADAR_SERVER_HTTP_IDLE_TIMEOUT`
- `PROTORADAR_SERVER_HTTP_MAX_HEADER_BYTES`

The outbox publisher is disabled by default. If you want Community to drain outbox records through the local logging dispatcher, set:

- `PROTORADAR_OUTBOX_PUBLISHER_ENABLED=true`
- `PROTORADAR_OUTBOX_PUBLISHER_BATCH_SIZE`
- `PROTORADAR_OUTBOX_PUBLISHER_POLL_INTERVAL`
- `PROTORADAR_OUTBOX_PUBLISHER_LEASE_DURATION`
- `PROTORADAR_OUTBOX_PUBLISHER_MAX_ATTEMPTS`
- `PROTORADAR_OUTBOX_PUBLISHER_INITIAL_BACKOFF`
- `PROTORADAR_OUTBOX_PUBLISHER_MAX_BACKOFF`

Community logging dispatch marks records `published` after bounded local logging. It does not deliver to Kafka/Sarama, webhooks, or any external event bus. Leave the publisher disabled if you want records to remain pending for a future real dispatcher.

During server shutdown, ProtoRadar cancels the outbox publisher and waits for it to stop before closing the database pool. That wait is bounded so shutdown cannot hang forever if a future dispatcher ignores cancellation. External dispatchers must honor context cancellation and set their own network timeouts; unfinished claimed records remain protected by the normal outbox lease/retry lifecycle.

Do not use `.env.example` secrets outside local development.

Environment variables override YAML files and use `PROTORADAR_<SECTION>_<FIELD>` names. Durations use values such as `5s`, `30s`, `1m`, or `2m30s`; size limits are numeric bytes only. Unknown YAML keys and invalid values fail startup validation instead of being ignored.

## CLI Image For CI

The server image and CLI image serve different purposes. The server image runs `protoradar-server` and includes migrations and Buf. The CLI image is built from the Dockerfile `cli` target, contains the `protoradar` CLI, and talks to a deployed ProtoRadar server through the REST API.

Build and smoke-test the CLI image locally:

```sh
make docker-build-cli CLI_IMAGE=protoradar-cli:local
make docker-smoke-cli CLI_IMAGE=protoradar-cli:local
```

Equivalent direct Docker commands:

```sh
docker build --target cli -t protoradar-cli:local .
docker run --rm protoradar-cli:local version
```

This repository does not publish an official public CLI image. For GitLab shared runners, build and push the image to your own registry, then set the GitLab CI/CD variable:

```yaml
variables:
  PROTORADAR_CLI_IMAGE: "registry.example.com/platform/protoradar-cli:v1.0.0"
```

The CLI image needs network reachability to `PROTORADAR_SERVER_URL`. If your server uses a private CA, bake the CA certificate into the CLI image or terminate TLS at a proxy trusted by the image.

## Health And Readiness

Use:

```text
GET /healthz
GET /readyz
```

`/healthz` is a process liveness check. `/readyz` checks required dependencies such as PostgreSQL and returns `503` when not ready.

## Metrics

Use:

```text
GET /metrics
```

Metrics are Prometheus text format and include HTTP request metrics, publish/breaking/runtime counters, and dependency graph gauges.

Avoid scraping labels with user input assumptions. ProtoRadar intentionally uses route templates, not raw paths.

When the outbox publisher is enabled, metrics also include claimed, dispatched, failed/dead, dispatch duration, and status gauge series for outbox records. Outbox metrics avoid record IDs, dedup keys, module names, and payload fields as labels.

## Logs

Configure:

```text
PROTORADAR_LOG_LEVEL=info
PROTORADAR_LOG_FORMAT=json
```

Supported formats are `json` and `text`; JSON is recommended for containers. Request logs include request IDs and route templates, and do not include Authorization headers or raw tokens.

The outbox publisher logs start/stop events, dispatch failures, shutdown timeout close errors, and records marked dead. The Community logging dispatcher logs record metadata and payload size only; it does not log full payloads.

## TLS And Authentication Boundary

Community v1.0 does not terminate TLS itself. Put ProtoRadar behind a reverse proxy or load balancer for HTTPS.

The REST API uses bearer API tokens. Governance uses authenticated API-token principal subjects as audit actors by default and does not provide OIDC, LDAP, advanced RBAC, or GitLab group sync. The Basic Web UI has no built-in login/RBAC in Community v1.0, so expose it only on trusted networks or behind an authentication proxy.

## Backup Notes

Back up:

- PostgreSQL database;
- S3/MinIO bucket contents;
- deployment configuration and secret references.

A restore should keep PostgreSQL artifact rows and S3 object keys aligned.

## Upgrade Notes

For v1.0, see [Upgrading](upgrading.md). From v1.0 onward, migrations are expected to be forward-compatible. Pre-v1.0 development schemas may require a fresh install or manual migration.
