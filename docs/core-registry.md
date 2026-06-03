# Core Registry MVP

The Core Registry MVP stores versioned protobuf modules. It is the foundation for later breaking-change checks, GitLab workflows, dependency analysis, and runtime inventory.

## What It Does

Phase 1 provides:

- versioned protobuf module storage;
- PostgreSQL metadata persistence;
- S3/MinIO artifact storage;
- REST API endpoints;
- CLI commands for publishing and pulling artifacts;
- API token authentication;
- transactional outbox records for outgoing domain events.

Published artifacts are tar.gz archives containing `.proto` files. The CLI creates these archives by walking the provided directory recursively and preserving paths relative to that directory.

## REST Endpoints

Authenticated with `Authorization: Bearer <token>`:

```text
POST /api/v1/modules
GET  /api/v1/modules
GET  /api/v1/modules/{module}
POST /api/v1/modules/{module}/versions
GET  /api/v1/modules/{module}/versions
GET  /api/v1/modules/{module}/versions/{version}
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

Publish a version:

```sh
protoradar push user-api \
  --version v1.0.0 \
  --path examples/user-api/proto
```

List modules:

```sh
protoradar list
```

Pull a version:

```sh
protoradar pull user-api \
  --version v1.0.0 \
  --output ./tmp/user-api
```

If the output directory already exists and is not empty, use `--force`.

## Artifact Safety

The CLI only includes `.proto` files in pushed artifacts. During pull, artifact extraction rejects absolute paths and traversal paths such as `../evil.proto`.

## Transactional Outbox

State-changing usecases write business data and outgoing event records in the same PostgreSQL transaction.

Events written in Phase 1:

- `ModuleCreated`
- `ModuleVersionPublished`

Usecases never publish directly to Kafka, Sarama, or another broker. Phase 1 persists outbox records only; publisher wiring is intentionally left for a later phase.
