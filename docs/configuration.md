# Configuration

ProtoRadar reads configuration from defaults, an optional YAML file, and environment variables. Defaults, YAML/env loading, validation, and typed runtime mapping live in `internal/config`; bootstrap only consumes validated typed values.

## Loading Order

Server configuration is loaded in this order:

1. Built-in defaults.
2. YAML file, when `protoradar-server --config <path>` is provided.
3. Environment variables, which override both defaults and YAML values.

If `--config` is omitted or passed as an empty path, the server uses defaults plus environment overrides. If `--config <path>` is provided, the file must exist and must be valid YAML.

Unknown YAML keys are rejected at startup. For example, `server.unsupported` fails with a config error instead of being ignored.

Environment variables use:

```text
PROTORADAR_<SECTION>_<FIELD>
```

Nested YAML fields are flattened with underscores. Examples:

- `server.http_read_timeout` -> `PROTORADAR_SERVER_HTTP_READ_TIMEOUT`
- `storage.s3.endpoint` -> `PROTORADAR_STORAGE_S3_ENDPOINT`
- `outbox_publisher.poll_interval` -> `PROTORADAR_OUTBOX_PUBLISHER_POLL_INTERVAL`

Some names are naturally short because the YAML path is short, such as `database.url` -> `PROTORADAR_DATABASE_URL`.

Before public usage, old pre-koanf env names such as `PROTORADAR_HTTP_ADDR` and `PROTORADAR_MAX_REQUEST_BODY_BYTES` were intentionally replaced. Use the names in this document; old names are not compatibility aliases.

## Value Formats

Durations use Go duration strings, for example `5s`, `30s`, `1m`, or `2m30s`. Duration fields must be positive.

Sizes and limits are numeric bytes only. Human-readable values such as `100MiB`, `110MB`, or `1m` are not supported for size fields.

Booleans use Go boolean parsing, for example `true`, `false`, `1`, or `0`.

URLs, addresses, tokens, and secrets remain strings.

## Validation And Secrets

Config validation runs before the server starts. Required strings must be non-empty, durations and sizes must be positive, enum-like fields must use documented values, and `outbox_publisher.initial_backoff` must be less than or equal to `outbox_publisher.max_backoff`.

Do not log environment values or whole environment dumps. Store tokens, S3 credentials, bootstrap tokens, GitLab tokens, and token hash secrets as masked CI/CD variables or deployment secrets. Do not commit real secrets to YAML files, `.env`, templates, or scripts.

## Local Example

Copy local defaults before starting Docker Compose:

```sh
cp .env.example .env
docker compose up --build
```

The values in `.env.example` are local placeholders, not production secrets.

Equivalent local-style YAML can be passed with `protoradar-server --config ./protoradar.yaml`:

```yaml
server:
  http_addr: :8080
  http_read_header_timeout: 5s
  http_read_timeout: 30s
  http_write_timeout: 60s
  http_idle_timeout: 120s
  http_max_header_bytes: 1048576
  max_request_body_bytes: 115343360
database:
  url: postgres://protoradar:protoradar@postgres:5432/protoradar?sslmode=disable
storage:
  s3:
    endpoint: http://minio:9000
    region: us-east-1
    bucket: protoradar
    access_key: minio
    secret_key: miniosecret
    use_path_style: true
auth:
  token_hash_secret: local-dev-token-hash-secret
  bootstrap_token: local-bootstrap-token
registry:
  max_artifact_size_bytes: 104857600
buf:
  binary_path: buf
  build_timeout: 30s
  lint_timeout: 30s
  lint_mode: warn
  require_config: true
  max_report_bytes: 16384
breaking:
  max_report_bytes: 32768
  max_changes: 1000
  default_against: latest
governance:
  enabled: true
  production_environments: production,prod
  allow_maintainer_approval: true
  actor_override_enabled: false
outbox_publisher:
  enabled: false
  batch_size: 50
  poll_interval: 5s
  lease_duration: 30s
  max_attempts: 5
  initial_backoff: 30s
  max_backoff: 5m
ui:
  enabled: true
  base_path: /ui
  static_path: /ui/static
log:
  level: info
  format: json
```

## Server

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_SERVER_HTTP_ADDR` | `server.http_addr` | no | `:8080` | HTTP listen address inside the server container/process. |
| `PROTORADAR_SERVER_HTTP_READ_HEADER_TIMEOUT` | `server.http_read_header_timeout` | no | `5s` | Maximum time to read request headers. Must be a positive duration. |
| `PROTORADAR_SERVER_HTTP_READ_TIMEOUT` | `server.http_read_timeout` | no | `30s` | Maximum time to read the full request, including the body. Must be a positive duration. |
| `PROTORADAR_SERVER_HTTP_WRITE_TIMEOUT` | `server.http_write_timeout` | no | `60s` | Maximum time to write an HTTP response. Must be a positive duration. |
| `PROTORADAR_SERVER_HTTP_IDLE_TIMEOUT` | `server.http_idle_timeout` | no | `120s` | Maximum keep-alive idle time between requests. Must be a positive duration. |
| `PROTORADAR_SERVER_HTTP_MAX_HEADER_BYTES` | `server.http_max_header_bytes` | no | `1048576` | Maximum bytes accepted for request headers. Must be positive. |
| `PROTORADAR_SERVER_MAX_REQUEST_BODY_BYTES` | `server.max_request_body_bytes` | no | `115343360` | Maximum HTTP request body size for upload/report endpoints. |

Docker Compose maps host `PROTORADAR_SERVER_HTTP_PORT` to container port `8080`; this is a Compose helper variable, not a server config field.

HTTP lifecycle durations are parsed and validated in `internal/config`, then wired into `net/http.Server` by bootstrap.

## Database

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_DATABASE_URL` | `database.url` | yes | none | PostgreSQL DSN. Local Compose uses `postgres://protoradar:protoradar@postgres:5432/protoradar?sslmode=disable`. |

Migrations run through Goose at server startup when migrations are available to the process. Goose stores migration state in `goose_db_version`; `schema_migrations` is not a supported migration state source.

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

## CLI Local Config

The CLI reads local config from `~/.config/protoradar/config.yaml` and also accepts environment overrides.

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_SERVER_URL` | `server_url` | yes for authenticated CLI commands | none | ProtoRadar server base URL used by the CLI. |
| `PROTORADAR_TOKEN` | `token` | yes for authenticated CLI commands | none | ProtoRadar API token used by the CLI. |
| `PROTORADAR_CLI_HTTP_TIMEOUT` | `http_timeout` | no | `30s` | Timeout for CLI HTTP calls to ProtoRadar and GitLab. Must be a positive duration. |

## Registry Limits

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_REGISTRY_MAX_ARTIFACT_SIZE_BYTES` | `registry.max_artifact_size_bytes` | no | `104857600` | Maximum accepted compressed source archive size. |

The request body limit should stay slightly larger than the artifact size limit to allow multipart form overhead. Server-side archive extraction also enforces the artifact limit as the maximum uncompressed source size. Size values are numeric bytes only.

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

## Governance

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_GOVERNANCE_ENABLED` | `governance.enabled` | no | `true` | Enable built-in governance policy evaluation. When disabled, approval requests can be created as `not_required`. |
| `PROTORADAR_GOVERNANCE_PRODUCTION_ENVIRONMENTS` | `governance.production_environments` | no | `production,prod` | Comma-separated environment names treated as production-like for affected consumer approval policy. |
| `PROTORADAR_GOVERNANCE_ALLOW_MAINTAINER_APPROVAL` | `governance.allow_maintainer_approval` | no | `true` | Allow maintainers, not only owners, to approve or reject requirements for their module. |
| `PROTORADAR_GOVERNANCE_ACTOR_OVERRIDE_ENABLED` | `governance.actor_override_enabled` | no | `false` | Allow legacy REST/CLI `actor` override values. Disabled by default so audit actors come from authenticated principals. |

Example YAML config:

```yaml
governance:
  enabled: true
  production_environments: production,prod,live
  allow_maintainer_approval: true
  actor_override_enabled: false
```

Community governance uses authenticated API-token principals as actor identity by default. Raw API tokens are not used as actor strings. Legacy caller-supplied actor override can be enabled for migration compatibility, but it weakens audit reliability because clients can request a different actor label than the authenticated principal. Keep `actor_override_enabled=false` for normal deployments; enable it only for local/dev compatibility or a controlled migration where callers are already trusted by another boundary. It does not configure OIDC, LDAP, advanced RBAC, GitLab group sync, or custom policy DSL.

## Outbox Publisher

| Env var | YAML field | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `PROTORADAR_OUTBOX_PUBLISHER_ENABLED` | `outbox_publisher.enabled` | no | `false` | Enable the transport-neutral outbox publisher loop. When disabled, outbox records remain `pending` until a publisher is enabled. |
| `PROTORADAR_OUTBOX_PUBLISHER_BATCH_SIZE` | `outbox_publisher.batch_size` | no | `50` | Maximum records to claim per publisher iteration. Must be greater than zero. |
| `PROTORADAR_OUTBOX_PUBLISHER_POLL_INTERVAL` | `outbox_publisher.poll_interval` | no | `5s` | Delay between publisher iterations. Must be a positive duration. |
| `PROTORADAR_OUTBOX_PUBLISHER_LEASE_DURATION` | `outbox_publisher.lease_duration` | no | `30s` | Claim lease duration for records in `processing`. Must be a positive duration. |
| `PROTORADAR_OUTBOX_PUBLISHER_MAX_ATTEMPTS` | `outbox_publisher.max_attempts` | no | `5` | Maximum dispatch attempts before a record is marked `dead`. Must be greater than zero. |
| `PROTORADAR_OUTBOX_PUBLISHER_INITIAL_BACKOFF` | `outbox_publisher.initial_backoff` | no | `30s` | Initial retry backoff after dispatch failure. Must be a positive duration. |
| `PROTORADAR_OUTBOX_PUBLISHER_MAX_BACKOFF` | `outbox_publisher.max_backoff` | no | `5m` | Maximum retry backoff. Must be a positive duration and at least the initial backoff. |

Example YAML config:

```yaml
outbox_publisher:
  enabled: true
  batch_size: 50
  poll_interval: 5s
  lease_duration: 30s
  max_attempts: 5
  initial_backoff: 30s
  max_backoff: 5m
```

Community Edition wires a `LoggingDispatcher`. It logs bounded outbox metadata and treats that local dispatch as success, so enabled Community publisher runs mark records `published` after local logging dispatch. It does not deliver records to Kafka/Sarama or any external event bus.

## Edition And Capabilities

Community Edition has no license key or enterprise-edition config in OSS. Bootstrap wires `CommunityCapabilityChecker`, which enables Community capabilities and reports represented enterprise capabilities as disabled.

Edition metadata is available through authenticated `GET /api/v1/edition`, `protoradar edition`, and `/ui/about`.

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

Outbox publisher metrics are emitted when the publisher runs:

- `protoradar_outbox_claimed_total`
- `protoradar_outbox_dispatch_total{status="published|failed|dead"}`
- `protoradar_outbox_dispatch_duration_seconds_sum`
- `protoradar_outbox_dispatch_duration_seconds_count`
- `protoradar_outbox_errors_total{status="..."}`
- `protoradar_outbox_pending_total`
- `protoradar_outbox_processing_total`
- `protoradar_outbox_failed_total`
- `protoradar_outbox_dead_total`

Outbox metric labels are bounded. They do not include record IDs, dedup keys, module names, payload fields, or other high-cardinality values.

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
| `PROTORADAR_CLI_HTTP_TIMEOUT` | Optional positive CLI HTTP timeout. Defaults to `30s`. |

Use environment auth in CI. Do not echo tokens in job logs.
