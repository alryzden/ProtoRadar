# Configuration

ProtoRadar reads configuration from environment variables or a YAML file passed to `protoradar-server --config <path>`. Defaults, env parsing, validation, and runtime parsing live in `internal/config`.

## Local Example

Copy local defaults before starting Docker Compose:

```sh
cp .env.example .env
docker compose up --build
```

The values in `.env.example` are local placeholders, not production secrets.

## Server

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_HTTP_ADDR` | `server.http_addr` | no | `:8080` | HTTP listen address inside the server container/process. |
| `PROTORADAR_MAX_REQUEST_BODY_BYTES` | `server.max_request_body_bytes` | no | `115343360` | Maximum HTTP request body size for upload/report endpoints. |

Docker Compose maps host `PROTORADAR_HTTP_PORT` to container port `8080`; this is a Compose variable, not a server config field.

## Database

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_DATABASE_URL` | `database.url` | yes | none | PostgreSQL DSN. Local Compose uses `postgres://protoradar:protoradar@postgres:5432/protoradar?sslmode=disable`. |

Migrations run at server startup when migrations are available to the process.

## S3 / MinIO Storage

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_STORAGE_S3_ENDPOINT` | `storage.s3.endpoint` | yes | none | S3-compatible endpoint, for example `http://minio:9000`. |
| `PROTORADAR_STORAGE_S3_REGION` | `storage.s3.region` | no | `us-east-1` | S3 region. |
| `PROTORADAR_STORAGE_S3_BUCKET` | `storage.s3.bucket` | yes | none | Bucket for source archives and Buf image artifacts. |
| `PROTORADAR_STORAGE_S3_ACCESS_KEY` | `storage.s3.access_key` | yes | none | S3 access key. |
| `PROTORADAR_STORAGE_S3_SECRET_KEY` | `storage.s3.secret_key` | yes | none | S3 secret key. |
| `PROTORADAR_STORAGE_S3_USE_PATH_STYLE` | `storage.s3.use_path_style` | no | `true` | Use path-style URLs. Required for local MinIO. |

Do not log or commit S3 credentials. Docker Compose creates the local bucket through the `minio-init` service.

## Authentication

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_AUTH_TOKEN_HASH_SECRET` | `auth.token_hash_secret` | yes | none | Secret used for API token HMAC hashing. |
| `PROTORADAR_AUTH_BOOTSTRAP_TOKEN` | `auth.bootstrap_token` | no | empty | Bootstrap bearer token for `POST /api/v1/tokens`. |

The API returns raw API tokens only once when created. PostgreSQL stores token hashes, not raw token values.

## Registry Limits

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_REGISTRY_MAX_ARTIFACT_SIZE_BYTES` | `registry.max_artifact_size_bytes` | no | `104857600` | Maximum accepted compressed source archive size. |

The request body limit should stay slightly larger than the artifact size limit to allow multipart form overhead. Server-side archive extraction also enforces the artifact limit as the maximum uncompressed source size.

## Buf Workflow

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_BUF_BINARY_PATH` | `buf.binary_path` | no | `buf` | Buf executable path. |
| `PROTORADAR_BUF_BUILD_TIMEOUT` | `buf.build_timeout` | no | `30s` | Timeout for `buf build` and breaking operations. |
| `PROTORADAR_BUF_LINT_TIMEOUT` | `buf.lint_timeout` | no | `30s` | Timeout for `buf lint`. |
| `PROTORADAR_BUF_LINT_MODE` | `buf.lint_mode` | no | `warn` | `disabled`, `warn`, or `enforce`. |
| `PROTORADAR_BUF_REQUIRE_CONFIG` | `buf.require_config` | no | `true` | Require `buf.yaml` at publish path root. |
| `PROTORADAR_BUF_MAX_REPORT_BYTES` | `buf.max_report_bytes` | no | `16384` | Maximum captured Buf output size for publish/lint diagnostics. |

## Breaking Checks

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_BREAKING_MAX_REPORT_BYTES` | `breaking.max_report_bytes` | no | `32768` | Maximum captured breaking-check output size. |
| `PROTORADAR_BREAKING_MAX_CHANGES` | `breaking.max_changes` | no | `1000` | Maximum parsed breaking changes retained. |
| `PROTORADAR_BREAKING_DEFAULT_AGAINST` | `breaking.default_against` | no | `latest` | Default baseline when CLI/API uses latest behavior. |

## Web UI

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_UI_ENABLED` | `ui.enabled` | no | `true` | Enable the Basic Web UI. |
| `PROTORADAR_UI_BASE_PATH` | `ui.base_path` | no | `/ui` | Web UI base path. |
| `PROTORADAR_UI_STATIC_PATH` | `ui.static_path` | no | `/ui/static` | Static asset path under the base path. |

The Web UI has no built-in login/RBAC in Community v1.0.

## Logs

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_LOG_LEVEL` | `log.level` | no | `info` | `debug`, `info`, `warn`, or `error`. |
| `PROTORADAR_LOG_FORMAT` | `log.format` | no | `json` | `json` or `text`. |

Request logs include method, route template, status, duration, request ID, and remote address. Authorization headers and raw tokens are not logged.

## Metrics

`GET /metrics` is exposed by the HTTP server in Prometheus text format. Metrics are always enabled in Community v1.0; there is no separate metrics config flag.

## CLI Auth Config

The CLI reads local config from:

```text
~/.config/protoradar/config.yaml
```

Environment overrides:

| Env var | Description |
| --- | --- |
| `PROTORADAR_SERVER_URL` | Server base URL. |
| `PROTORADAR_TOKEN` | API token. |

Use environment auth in CI. Do not echo tokens in job logs.
