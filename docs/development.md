# Development

## Local Stack

Run the full local stack:

```sh
docker compose up
```

Services:

- `protoradar-server`: `http://localhost:8080`
- PostgreSQL: `localhost:5432`
- MinIO API: `http://localhost:9000`
- MinIO console: `http://localhost:9001`

The `minio-init` service creates the `protoradar` bucket if it does not already exist. The server image includes the Buf CLI so publish can run server-side `buf build` and optional `buf lint`.

## Local Environment

Docker Compose configures:

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

Buf config ownership lives in `internal/config`. Bootstrap receives typed runtime values and only wires the concrete Buf CLI adapter.

## Building

Build the server and CLI:

```sh
go build ./cmd/protoradar-server
go build ./cmd/protoradar
```

Run tests:

```sh
go test ./...
```

## Example Publish

```sh
protoradar login \
  --server http://localhost:8080 \
  --token <token>

protoradar module create user-api \
  --description "User service protobuf contracts" \
  --repository-url "https://gitlab.example.com/platform/user-api"

protoradar push user-api \
  --version v1.1.0 \
  --path examples/user-api
```

The `examples/user-api` directory is a Buf module root with `buf.yaml` and protobuf sources under `proto/`.

## Registry Development Rules

Keep the clean architecture boundaries intact:

- domain contains business vocabulary and repository ports only;
- usecases orchestrate state changes and write outbox records;
- repository/postgres contains PostgreSQL implementations;
- infrastructure/bufcli contains concrete Buf CLI execution;
- infrastructure/objectstorage/s3 contains the S3/MinIO adapter;
- transport/http contains HTTP DTOs and handlers;
- internal/cli talks only through the REST API;
- app/bootstrap wires concrete dependencies only.

Usecases must not publish to Kafka/Sarama. For state-changing registry operations, write an `outbox.Record` in the same transaction as business state changes.

## Known Limitations

- Breaking-change comparison is not implemented yet.
- Dependency graph UI is not implemented yet.
- Generated SDK workflows are not implemented yet.
