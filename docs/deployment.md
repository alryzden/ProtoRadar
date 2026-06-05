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

PostgreSQL stores all relational state: modules, versions, artifact metadata, descriptor metadata, reports, dependency graph, runtime inventory, API token hashes, and outbox records.

Requirements:

- Use persistent storage for the PostgreSQL data directory.
- Back up the database regularly.
- Run only one server instance applying migrations during startup unless you have deployment coordination.

Migrations run at server startup from the `migrations/` directory packaged in the image.

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

Do not use `.env.example` secrets outside local development.

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

## Logs

Configure:

```text
PROTORADAR_LOG_LEVEL=info
PROTORADAR_LOG_FORMAT=json
```

Supported formats are `json` and `text`; JSON is recommended for containers. Request logs include request IDs and route templates, and do not include Authorization headers or raw tokens.

## TLS And Authentication Boundary

Community v1.0 does not terminate TLS itself. Put ProtoRadar behind a reverse proxy or load balancer for HTTPS.

The REST API uses bearer API tokens. The Basic Web UI has no built-in login/RBAC in Community v1.0, so expose it only on trusted networks or behind an authentication proxy.

## Backup Notes

Back up:

- PostgreSQL database;
- S3/MinIO bucket contents;
- deployment configuration and secret references.

A restore should keep PostgreSQL artifact rows and S3 object keys aligned.

## Upgrade Notes

For v1.0, see [Upgrading](upgrading.md). From v1.0 onward, migrations are expected to be forward-compatible. Pre-v1.0 development schemas may require a fresh install or manual migration.
