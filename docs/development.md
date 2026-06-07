# Development

## Local Stack

Run the full local stack:

```sh
cp .env.example .env
docker compose up --build
```

Services:

- `protoradar-server`: `http://localhost:8080`
- Basic Web UI: `http://localhost:8080/ui`
- PostgreSQL: `localhost:5432`
- MinIO API: `http://localhost:9000`
- MinIO console: `http://localhost:9001`

The `minio-init` service creates the `protoradar` bucket if it does not already exist. The server image includes the Buf CLI so publish can run server-side `buf build`, optional `buf lint`, and `buf breaking`.

Run quick local verification after the stack is ready:

```sh
make smoke-test
```

Run the local demo workflow:

```sh
make demo
```

## Local Environment

Docker Compose configures:

```text
PROTORADAR_SERVER_HTTP_ADDR=:8080
PROTORADAR_SERVER_HTTP_READ_HEADER_TIMEOUT=5s
PROTORADAR_SERVER_HTTP_READ_TIMEOUT=30s
PROTORADAR_SERVER_HTTP_WRITE_TIMEOUT=60s
PROTORADAR_SERVER_HTTP_IDLE_TIMEOUT=120s
PROTORADAR_SERVER_HTTP_MAX_HEADER_BYTES=1048576
PROTORADAR_SERVER_MAX_REQUEST_BODY_BYTES=115343360
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
PROTORADAR_GOVERNANCE_ENABLED=true
PROTORADAR_GOVERNANCE_PRODUCTION_ENVIRONMENTS=production,prod
PROTORADAR_GOVERNANCE_ALLOW_MAINTAINER_APPROVAL=true
PROTORADAR_GOVERNANCE_ACTOR_OVERRIDE_ENABLED=false
PROTORADAR_OUTBOX_PUBLISHER_ENABLED=false
PROTORADAR_OUTBOX_PUBLISHER_BATCH_SIZE=50
PROTORADAR_OUTBOX_PUBLISHER_POLL_INTERVAL=5s
PROTORADAR_OUTBOX_PUBLISHER_LEASE_DURATION=30s
PROTORADAR_OUTBOX_PUBLISHER_MAX_ATTEMPTS=5
PROTORADAR_OUTBOX_PUBLISHER_INITIAL_BACKOFF=30s
PROTORADAR_OUTBOX_PUBLISHER_MAX_BACKOFF=5m
PROTORADAR_UI_ENABLED=true
PROTORADAR_UI_BASE_PATH=/ui
PROTORADAR_UI_STATIC_PATH=/ui/static
PROTORADAR_LOG_LEVEL=info
PROTORADAR_LOG_FORMAT=json
```

Buf config ownership lives in `internal/config`. Bootstrap receives typed runtime values and only wires the concrete Buf CLI adapter.

Local observability endpoints:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

## Building

This repository targets Go 1.26. Keep `go.mod`, Dockerfile, CI, and docs aligned with Go 1.26.

Build the server and CLI:

```sh
make build
```

Build the local CLI container image used by GitLab CI examples:

```sh
make docker-build-cli CLI_IMAGE=protoradar-cli:local
```

Equivalent direct Docker command:

```sh
docker build --target cli -t protoradar-cli:local .
```

Smoke test the image:

```sh
make docker-smoke-cli CLI_IMAGE=protoradar-cli:local
docker run --rm protoradar-cli:local version
```

The CLI image contains only `protoradar`; it uses REST API authentication with `PROTORADAR_SERVER_URL` and `PROTORADAR_TOKEN`. The repository does not publish a public CLI image, so GitLab shared-runner usage requires pushing your built image to a registry and setting `PROTORADAR_CLI_IMAGE`.

Run tests:

```sh
make test
make vet
```

The architecture/import-boundary tests live in `internal/archtest` and run through the same command path:

```sh
go test ./...
```

They do not require PostgreSQL, MinIO/S3, Docker, Buf CLI, GitLab, Kafka/Sarama, or network access. If they fail, treat the failure as an architecture regression unless the import is a deliberate, narrow boundary exception.

Format Go files:

```sh
make fmt
```

Run race tests when practical:

```sh
make test-race
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
  --version v1.0.0 \
  --path examples/repos/user-api
```

The `examples/repos/user-api` directory is a Buf module root with `buf.yaml` and protobuf sources under `proto/`.

Open `http://localhost:8080/ui/modules` to inspect the module list, module details, published version artifacts, Buf config, and descriptor metadata. See `docs/web-ui.md` for the full UI workflow.

## Example Breaking Check

After publishing a baseline, edit `examples/repos/user-api/proto/user/v1/user.proto` in a breaking way, such as changing a field type or removing an RPC. Then run:

```sh
protoradar check-breaking user-api \
  --path examples/repos/user-api \
  --against latest
```

Expected result for a breaking edit:

- CLI prints a `ProtoRadar Breaking Change Report`;
- process exits with code `1`;
- server stores a breaking report and writes `BreakingReportCreated` through the transactional outbox.

Open `http://localhost:8080/ui/breaking-reports` to inspect stored breaking reports and report details.

## Example Runtime Inventory

After publishing module versions, report a deployed service snapshot:

```sh
protoradar runtime report \
  --service billing-service \
  --environment production \
  --git-commit abc1234 \
  --build-version 2026.06.04-15 \
  --module user-api@v1.0.0
```

Or use the example file:

```sh
protoradar runtime report --from-file examples/runtime/protoradar-runtime.yaml
```

Open these UI pages to inspect runtime inventory:

- `http://localhost:8080/ui/runtime/services`
- `http://localhost:8080/ui/runtime/environments/production`
- `http://localhost:8080/ui/modules/user-api/runtime-usages`

Breaking report detail pages include Runtime impact when services report usage of the affected base module version.

## GitLab CI Examples

GitLab CI templates live under `examples/gitlab`. They are documentation/examples only and do not change the local Docker Compose stack.

Use them to exercise:

- merge-request breaking checks with `protoradar check-breaking`;
- tag-based publishing with `protoradar push`;
- post-deploy runtime inventory reporting with `protoradar runtime report`;
- module-to-GitLab project mapping with `protoradar module link-gitlab`.

The reusable templates require `PROTORADAR_CLI_IMAGE`, `PROTORADAR_SERVER_URL`, `PROTORADAR_TOKEN`, and `PROTORADAR_MODULE`. Store tokens as masked GitLab CI/CD variables and do not echo them in scripts.

See `docs/gitlab-ci.md` for the full CI workflow.

## Migrations

Migrations live in `migrations/` and are applied by Goose during server startup when the migrations filesystem is available. For local Docker Compose, starting `protoradar-server` against an empty database creates the schema.

Before adding a migration, check `git status` and existing files in `migrations/`. If an uncommitted migration belongs to the same feature, update that migration pair instead of creating a fix-up migration.

Keep Goose annotations valid in `*.up.sql` and `*.down.sql`. Runtime execution combines each matching up/down pair for Goose without renaming the source files.

Goose records runtime migration state in `goose_db_version`. ProtoRadar does not support or seed the old custom `schema_migrations` state table.

PostgreSQL repository and migration-runner tests start PostgreSQL automatically with Testcontainers when Docker is available. The PostgreSQL repository package uses one shared Testcontainers PostgreSQL container per package test run, while each test gets its own isolated schema, pgx pool search path, and Goose migration state. No manually prepared external test database is required; Docker is required to execute the self-contained database integration tests locally. When Docker or the Testcontainers provider is unavailable, database-backed tests skip with a clear unavailable reason.

## Code Style And Architecture Rules

Keep the clean architecture boundaries intact:

- domain contains business vocabulary and repository ports only;
- usecases orchestrate state changes and write outbox records;
- repository/postgres contains PostgreSQL implementations;
- infrastructure/bufcli contains concrete Buf CLI execution;
- infrastructure/objectstorage/s3 contains the S3/MinIO adapter;
- transport/http contains HTTP DTOs and handlers;
- transport/web contains read-only HTML handlers and templates;
- internal/cli stays REST-client oriented and does not import server repositories or transports;
- app/bootstrap wires concrete dependencies only.

`internal/archtest` enforces these rules for production imports where they can be checked mechanically:

- domain cannot import infrastructure, transport, config, app/bootstrap, repository/postgres, SQL/pgx, S3/MinIO SDKs, Buf adapters, GitLab concrete clients, Kafka/Sarama, or enterprise package paths;
- usecases cannot import concrete infrastructure, HTTP/Web transport, CLI code, repository/postgres, concrete S3/Buf/GitLab adapters, Kafka/Sarama, topic routing, SQL/pgx, or enterprise package paths;
- HTTP and Web transports cannot import repository/postgres directly;
- CLI code stays REST-client oriented, except for the existing `gitlab mr-check` local GitLab MR orchestration exception;
- outbox remains transport-neutral;
- identity and authorization stay transport-free and concrete-provider-free;
- OSS core does not import enterprise packages;
- raw server config parsing stays in `internal/config` where simple AST checks can enforce it.

When adding a new package, route, adapter, or workflow:

- add domain vocabulary and repository ports under `internal/domain` only when they are business concepts;
- add application orchestration and interfaces under `internal/usecase` or another application-level boundary package;
- put concrete PostgreSQL, S3, Buf, GitLab, Kafka/Sarama, HTTP, Web, or CLI integrations in repository, infrastructure, transport, or CLI packages as appropriate;
- wire concrete implementations in `internal/app/bootstrap` or a downstream/private composition root;
- keep raw server config defaults, env parsing, validation, and duration parsing in `internal/config`;
- update `internal/archtest` only for a legitimate, narrow exception and document the reason in the allowlist;
- avoid broad allowlists such as an entire layer importing `internal/infrastructure`, because that makes the test meaningless.

Phase 11 open-core extension points:

- `internal/identity.AuthProvider` returns principals and owns authentication provider boundaries.
- `internal/authorization.Authorizer` owns action/resource authorization checks.
- `internal/audit.AuditSink` owns product audit recording.
- `internal/usecase/governance.ApprovalWorkflow` owns approval workflow replacement.
- `internal/edition.CapabilityChecker` and `Edition` own capability and edition reporting.

Downstream enterprise packages may depend on these OSS interfaces. OSS core must not import enterprise packages or mention enterprise concrete types in interfaces. Wire alternate implementations in the downstream composition root.

When adding HTTP API routes, classify the route before writing handler code:

- public: operational endpoints only, currently `GET /healthz`, `GET /readyz`, and `GET /metrics`;
- bootstrap-only: intentionally special bootstrap-token flows, currently `POST /api/v1/tokens`;
- protected: normal API routes under `/api/v1`.

Public and bootstrap-only routes are exceptions, not the default. Document why the route does not use normal `Authorizer` denial, add it to the route classification table, and add or update API docs if caller-visible behavior changes.

Protected routes must use the HTTP `protected(action, resource, handler)` wrapper, not direct `requireBearer`. Pick an existing authorization action where it matches the operation, or add a minimal action in `internal/authorization` and include it in `CommunityAuthorizer` if Community behavior should allow it. Resources should identify the target module, module version, breaking report, runtime object, governance object, approval request, or edition metadata without leaking transport-specific types into the authorization package.

Tests for new protected routes must prove:

- missing or invalid auth returns unauthorized before authorization;
- a denying `Authorizer` blocks the route with forbidden;
- an allowing `Authorizer` reaches handler validation or normal handler behavior;
- expected action/resource values are recorded by the test authorizer;
- the route is included in the HTTP route authorization coverage table.

When changing HTTP routes, update the test guardrails in `internal/transport/http/server_test.go`:

- add protected routes to `authenticatedRouteExpectations` with method, path, body if needed, expected action, and expected resource;
- keep focused route-group tables readable when a route belongs to registry, version, artifact, metadata, breaking-report, GitLab mapping, dependency graph, runtime inventory, edition, or governance coverage;
- ensure `routeClassificationExpectations` classifies every registered route as public, bootstrap-only, or protected;
- keep public-route tests proving no Authorization header is required and `Authorizer` is not called;
- keep bootstrap-token tests proving `POST /api/v1/tokens` uses the bootstrap token path and not normal bearer `Authorizer` denial;
- update `docs/api.md` when public/bootstrap behavior or route authentication semantics change.

Allowed-path authorization tests may assert validation, conflict, or not-found responses when fake dependencies intentionally avoid PostgreSQL, S3, Buf, GitLab, Kafka, or network access. Do not weaken them to broad `status != 500` checks; assert the expected stable response for that fake setup and separately assert that the fake `Authorizer` recorded the exact action/resource.

Usecases must not publish to Kafka/Sarama. For state-changing registry operations, write an `outbox.Record` in the same transaction as business state changes.

Development rule files are the source of truth for contributors and automated coding tools:

- `AGENTS.md`
- `docs/development/architecture.md`
- `docs/development/go-style.md`
- `docs/development/configuration.md`
- `docs/development/event-flow.md`
- `docs/development/production-safety.md`
- `docs/development/readability.md`
- `docs/development/testing.md`
- `docs/development/review-checklist.md`

## Known Limitations

- Dependency graph analysis is direct-only; transitive traversal is not implemented yet.
- Waiver workflows are not implemented yet.
- Enterprise auth, advanced RBAC, license management, GitLab group sync, and advanced approval workflows are not implemented in OSS.
- Breaking diagnostic parsing is best-effort.
- Web UI login/RBAC is not implemented yet.
- Runtime inventory reports are deployment snapshots, not continuous heartbeats.
- Generated-client usage is not modeled yet.
- Generated SDK workflows are not implemented yet.
- Authorizer denial tests prove HTTP routes invoke `Authorizer`; they do not prove `CommunityAuthorizer` enforces strict RBAC.
- Authorization route tests do not replace policy review when adding new actions or resources.
- Some allow-path HTTP unit tests intentionally return validation, conflict, or not-found responses after authorization because external services and real persistence are not used.
