# ProtoRadar

ProtoRadar is a self-hosted protobuf governance platform. Phase 2 adds a Buf-compatible publish workflow on top of the Core Registry: publishing a module version validates uploaded protobuf sources with server-side Buf, stores both source and Buf image artifacts, extracts descriptor metadata, and emits a transactional outbox event for downstream processing.

## What It Does

ProtoRadar currently supports:

- module creation and listing;
- Buf-compatible version publishing;
- server-side `buf build`;
- optional `buf lint` in `disabled`, `warn`, or `enforce` mode;
- source archive and Buf image artifact storage;
- descriptor metadata persistence for files, imports, services, methods, messages, fields, enums, and enum values;
- descriptor metadata retrieval through REST;
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
  --version v1.1.0 \
  --path examples/user-api
```

List modules:

```sh
protoradar list
```

View descriptor metadata:

```sh
curl -sS \
  -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/modules/user-api/versions/v1.1.0/metadata
```

Pull the source archive:

```sh
protoradar pull user-api \
  --version v1.1.0 \
  --output ./tmp/user-api
```

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

Known limitations:

- no breaking-change comparison yet;
- no dependency graph UI yet;
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

`ModuleVersionPublished` is written inside the same PostgreSQL transaction as version, artifact, Buf config, and descriptor metadata records. Kafka/Sarama routing and publishing remain outside usecases.

## More Documentation

- [Buf Workflow](docs/buf-workflow.md)
- [Core Registry](docs/core-registry.md)
- [Development](docs/development.md)
