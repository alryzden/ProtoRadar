# Core Registry

The Core Registry stores versioned protobuf modules and their publish metadata. Phase 2 makes publish Buf-compatible: uploaded sources are validated server-side with Buf before the module version is persisted.

## What It Does

The registry provides:

- versioned protobuf module storage;
- PostgreSQL metadata persistence;
- S3/MinIO object storage;
- REST API endpoints;
- CLI commands for publishing and pulling artifacts;
- API token authentication;
- transactional outbox records for outgoing domain events;
- Buf config metadata persistence;
- descriptor metadata persistence.

Published versions now store two artifact kinds:

- `source_archive`: the uploaded tar.gz archive containing `buf.yaml`, optional `buf.lock`, and `.proto` files;
- `buf_image`: the server-built binary descriptor image produced by Buf.

## REST Endpoints

Authenticated with `Authorization: Bearer <token>`:

```text
POST /api/v1/modules
GET  /api/v1/modules
GET  /api/v1/modules/{module}
POST /api/v1/modules/{module}/versions
GET  /api/v1/modules/{module}/versions
GET  /api/v1/modules/{module}/versions/{version}
GET  /api/v1/modules/{module}/versions/{version}/metadata
GET  /api/v1/modules/{module}/versions/{version}/artifact
```

Bootstrap-token protected:

```text
POST /api/v1/tokens
```

Public:

```text
GET /healthz
GET /readyz
```

## CLI Workflow

Create an API token with the bootstrap token:

```sh
curl -sS \
  -H "Authorization: Bearer local-bootstrap-token" \
  -H "Content-Type: application/json" \
  -d '{"name":"local-cli"}' \
  http://localhost:8080/api/v1/tokens
```

Log in:

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

Publish a version from a Buf module root:

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

Pull a source archive:

```sh
protoradar pull user-api \
  --version v1.1.0 \
  --output ./tmp/user-api
```

If the output directory already exists and is not empty, use `--force`.

## Artifact Safety

The CLI packages only `buf.yaml`, optional `buf.lock`, and `.proto` files. It preserves relative paths and excludes local/build directories such as `.git`, `node_modules`, `tmp`, `dist`, `build`, `generated`, and `vendor`.

On the server, source archive extraction rejects absolute paths, traversal paths such as `../evil.proto`, symlinks, hardlinks, non-regular entries, and archives that exceed the configured uncompressed size limit.

## Descriptor Metadata

The server extracts and persists descriptor metadata from the Buf image:

- files and package names;
- imports;
- services and methods;
- messages and fields;
- enums and enum values;
- summary counts.

This metadata is intended for later breaking-change checks and dependency analysis.

## Transactional Outbox

State-changing usecases write business data and outgoing event records in the same PostgreSQL transaction.

Events currently written:

- `ModuleCreated`
- `ModuleVersionPublished`

`ModuleVersionPublished` is written after successful Buf build and inside the same transaction as version, artifact, Buf config, and descriptor metadata records. Usecases never publish directly to Kafka, Sarama, or another broker.
