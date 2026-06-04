# ProtoRadar

ProtoRadar is a self-hosted protobuf governance platform. It provides a Buf-compatible registry workflow: publishing a module version validates protobuf sources with server-side Buf, stores source and Buf image artifacts, extracts descriptor metadata, and emits transactional outbox events for downstream processing. It supports server-side breaking-change checks against previously published Buf images and GitLab CI workflows for merge-request validation, merge-request bot comments/statuses, tag-based publishing, and module-to-project mapping.

## What It Does

ProtoRadar currently supports:

- module creation and listing;
- Buf-compatible version publishing;
- server-side `buf build`;
- optional `buf lint` in `disabled`, `warn`, or `enforce` mode;
- source archive and Buf image artifact storage;
- descriptor metadata persistence for files, imports, services, methods, messages, fields, enums, and enum values;
- descriptor metadata retrieval through REST;
- direct dependency graph detection for imports, field types, and RPC input/output types;
- affected-module queries for direct downstream consumers;
- breaking-change checks against published versions;
- GitLab CI templates for merge-request checks, merge-request bot comments/statuses, and tag publishing;
- GitLab project mapping for ProtoRadar modules;
- read-only Basic Web UI for local/demo inspection;
- API token authentication;
- a `protoradar` CLI;
- durable outgoing event records through a transactional outbox.

State-changing usecases do not publish to Kafka or any external broker directly. They write outbox records in the same database transaction as the business state change.

## Quickstart

Start the local stack:

```sh
docker compose up
```

The compose stack starts:

- `protoradar-server` on `http://localhost:8080`
- PostgreSQL on `localhost:5432`
- MinIO on `localhost:9000`
- MinIO console on `http://localhost:9001`

The server image includes the Buf CLI for server-side validation. The local bootstrap token is configured in `docker-compose.yml` as:

```text
local-bootstrap-token
```

Create an API token:

```sh
curl -sS \
  -H "Authorization: Bearer local-bootstrap-token" \
  -H "Content-Type: application/json" \
  -d '{"name":"local-cli"}' \
  http://localhost:8080/api/v1/tokens
```

Use the returned `token` value to log in:

```sh
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

Publish the example Buf module:

```sh
protoradar push user-api \
  --version v1.0.0 \
  --path examples/user-api
```

Check a proposed workspace against the latest published baseline:

```sh
protoradar check-breaking user-api \
  --path examples/user-api \
  --against latest
```

Link the module to a GitLab project:

```sh
protoradar module link-gitlab user-api \
  --project-id 12345 \
  --project-path platform/user-api \
  --gitlab-base-url https://gitlab.example.com
```

List modules:

```sh
protoradar list
```

View descriptor metadata:

```sh
curl -sS \
  -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/modules/user-api/versions/v1.0.0/metadata
```

Pull the source archive:

```sh
protoradar pull user-api \
  --version v1.0.0 \
  --output ./tmp/user-api
```

Open the Basic Web UI:

```text
http://localhost:8080/ui
```

The UI is read-only and shows modules, versions, artifacts, descriptor metadata, and breaking reports. See [Basic Web UI](docs/web-ui.md).

Inspect direct dependency graph data:

```sh
protoradar module dependencies user-api
protoradar module affected user-api
```

See [Dependency Graph MVP](docs/dependency-graph.md) for example modules and graph behavior.

## Buf-Compatible Workflow

Publishing requires `buf.yaml` at the root of the path passed to `protoradar push`. If `buf.lock` is present, the CLI includes it and the server records its presence and digest.

The server extracts the uploaded source archive safely, runs `buf build`, stores the resulting Buf image, optionally runs `buf lint`, extracts descriptor metadata, and persists all publish metadata transactionally.

Lint modes:

- `disabled`: skip lint;
- `warn`: allow publish and return warning status/report;
- `enforce`: reject publish when lint fails.

Stored artifacts:

- `source_archive`: uploaded tar.gz source archive;
- `buf_image`: server-built binary descriptor image.

## Breaking Change Checks

Breaking checks compare a proposed Buf workspace against a baseline version's stored `buf_image` artifact. The baseline must already be published, and the proposed workspace must contain `buf.yaml`.

CLI examples:

```sh
protoradar check-breaking user-api --path . --against latest
protoradar check-breaking user-api --path . --against v1.0.0
protoradar check-breaking user-api --path . --against latest --target-ref feature/user-api
```

Stable exit codes:

- `0`: no breaking changes;
- `1`: breaking changes found;
- `2`: input, auth, network, server, tool, config, or internal error.

REST endpoints:

- `POST /api/v1/modules/{module}/breaking-checks`
- `GET /api/v1/breaking-reports/{report_id}`
- `GET /api/v1/modules/{module}/breaking-reports`
- `GET /api/v1/breaking-reports/{report_id}/affected-modules`

Reports include `status`, `change_count`, structured `changes`, and `human_summary`. Breaking changes are normal check results and return HTTP `200`; they are not treated as server failures.

## GitLab CI

ProtoRadar includes reusable GitLab CI templates under `examples/gitlab`:

- `protoradar-breaking-check.yml`: simple merge-request breaking-change validation without GitLab API access;
- `protoradar-mr-check.yml`: merge-request bot comments and commit status;
- `protoradar-publish.yml`: tag-based module publishing;
- `protoradar-full.yml`: combined MR bot/publish example.

CI jobs use CLI environment authentication for ProtoRadar:

```text
PROTORADAR_SERVER_URL
PROTORADAR_TOKEN
PROTORADAR_MODULE
```

The MR bot template also requires `PROTORADAR_GITLAB_TOKEN` so the CLI can create or update merge request comments and set commit statuses. Store `PROTORADAR_TOKEN` and `PROTORADAR_GITLAB_TOKEN` in GitLab CI/CD variables and mark them masked. Do not commit tokens to the repository.

See [GitLab CI Integration](docs/gitlab-ci.md), [GitLab Merge Request Bot](docs/gitlab-mr-bot.md), and [GitLab examples](examples/gitlab/README.md).

## Basic Web UI

The server includes a read-only Web UI for local demos and internal inspection. It is enabled by default at:

```text
http://localhost:8080/ui
```

Use it to browse modules, inspect published version artifacts and protobuf descriptor metadata, review stored breaking reports, and inspect direct dependency graph data. The UI does not include login/RBAC or write operations, so do not expose it publicly without authentication or a trusted reverse proxy.

See [Basic Web UI](docs/web-ui.md).

## Dependency Graph

ProtoRadar detects direct module dependencies during publish from import paths, field type references, and RPC input/output type references. It can show upstream dependencies, downstream consumers, unresolved dependencies, and modules that may be affected by changes to a provider module.

CLI examples:

```sh
protoradar module dependencies user-api
protoradar module affected user-api
```

REST endpoints:

- `GET /api/v1/modules/{module}/dependencies`
- `GET /api/v1/modules/{module}/affected`
- `GET /api/v1/breaking-reports/{report_id}/affected-modules`

The Basic Web UI exposes `/ui/modules/{module}/dependencies`, and GitLab MR comments include potentially affected modules when known.

See [Dependency Graph MVP](docs/dependency-graph.md).

Known limitations:

- direct dependencies only; no transitive traversal yet;
- no runtime usage detection yet;
- no approval workflow yet;
- breaking diagnostic parsing is best-effort;
- no login/RBAC for the Web UI yet;
- no generated SDKs yet.

## Configuration

The server reads config from environment variables or a YAML file. For local development, Docker Compose sets:

```text
PROTORADAR_HTTP_ADDR=:8080
PROTORADAR_DATABASE_URL=postgres://protoradar:protoradar@postgres:5432/protoradar?sslmode=disable
PROTORADAR_STORAGE_S3_ENDPOINT=http://minio:9000
PROTORADAR_STORAGE_S3_REGION=us-east-1
PROTORADAR_STORAGE_S3_BUCKET=protoradar
PROTORADAR_STORAGE_S3_ACCESS_KEY=minio
PROTORADAR_STORAGE_S3_SECRET_KEY=miniosecret
PROTORADAR_STORAGE_S3_USE_PATH_STYLE=true
PROTORADAR_AUTH_TOKEN_HASH_SECRET=local-dev-token-hash-secret
PROTORADAR_AUTH_BOOTSTRAP_TOKEN=local-bootstrap-token
PROTORADAR_REGISTRY_MAX_ARTIFACT_SIZE_BYTES=104857600
PROTORADAR_BUF_BINARY_PATH=buf
PROTORADAR_BUF_BUILD_TIMEOUT=30s
PROTORADAR_BUF_LINT_TIMEOUT=30s
PROTORADAR_BUF_LINT_MODE=warn
PROTORADAR_BUF_REQUIRE_CONFIG=true
PROTORADAR_BUF_MAX_REPORT_BYTES=16384
PROTORADAR_BREAKING_MAX_REPORT_BYTES=32768
PROTORADAR_BREAKING_MAX_CHANGES=1000
PROTORADAR_BREAKING_DEFAULT_AGAINST=latest
PROTORADAR_UI_ENABLED=true
PROTORADAR_UI_BASE_PATH=/ui
PROTORADAR_UI_STATIC_PATH=/ui/static
```

The CLI stores local credentials in:

```text
~/.config/protoradar/config.yaml
```

The file is written with `0600` permissions. CLI environment overrides are:

```text
PROTORADAR_SERVER_URL
PROTORADAR_TOKEN
```

## API Tokens

`POST /api/v1/tokens` is protected by the bootstrap token. The API returns the raw token once. The server stores only an HMAC-SHA256 token hash. Raw token values are not included in outbox events.

The CLI stores the raw token locally because it must send it as a bearer token on later requests. Treat the CLI config file as a credential.

## Transactional Outbox

Current outgoing event records:

- `protoradar.module.created` for `ModuleCreated`
- `protoradar.module_version.published` for `ModuleVersionPublished`
- `protoradar.breaking_report.created` for `BreakingReportCreated`
- `protoradar.module_gitlab_project.linked` for `ModuleGitLabProjectLinked`
- `protoradar.module_dependencies.updated` for `ModuleDependenciesUpdated`

`ModuleVersionPublished` is written inside the same PostgreSQL transaction as version, artifact, Buf config, and descriptor metadata records. Kafka/Sarama routing and publishing remain outside usecases.

`BreakingReportCreated` is written inside the same PostgreSQL transaction as the breaking report and change records. Usecases never publish directly to Kafka, Sarama, or any broker.

`ModuleGitLabProjectLinked` is written inside the same PostgreSQL transaction as the module-to-GitLab project mapping.

`ModuleDependenciesUpdated` is written inside the same transaction as dependency graph records for a published module version.

## More Documentation

- [Buf Workflow](docs/buf-workflow.md)
- [Breaking Checks](docs/breaking-checks.md)
- [Core Registry](docs/core-registry.md)
- [Dependency Graph MVP](docs/dependency-graph.md)
- [GitLab CI Integration](docs/gitlab-ci.md)
- [GitLab Merge Request Bot](docs/gitlab-mr-bot.md)
- [Basic Web UI](docs/web-ui.md)
- [Development](docs/development.md)
