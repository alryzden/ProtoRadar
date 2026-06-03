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

The `minio-init` service creates the `protoradar` bucket if it does not already exist.

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
```

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

## Registry Development Rules

Keep the clean architecture boundaries intact:

- domain contains business vocabulary and repository ports only;
- usecases orchestrate state changes and write outbox records;
- repository/postgres contains PostgreSQL implementations;
- infrastructure/objectstorage/s3 contains the S3/MinIO adapter;
- transport/http contains HTTP DTOs and handlers;
- app/bootstrap wires concrete dependencies only.

Usecases must not publish to Kafka/Sarama. For state-changing registry operations, write an `outbox.Record` in the same transaction as business state changes.
