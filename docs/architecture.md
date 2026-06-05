# Architecture

ProtoRadar follows clean architecture with explicit package boundaries. Business state changes happen in usecases, persistence is behind repository ports, and external systems are wired in bootstrap.

## Text Diagram

```text
CLI / HTTP clients / GitLab CI
        |
        v
internal/transport/http   internal/transport/web   internal/cli
        |                         |                  |
        v                         v                  v
internal/usecase/registry   internal/usecase/uiquery   REST API client
internal/usecase/dependencygraph
internal/usecase/runtimeinventory
        |
        v
internal/domain ports and business vocabulary
        |
        v
internal/repository/postgres ---- PostgreSQL
        |
        v
internal/outbox durable records

Infrastructure adapters:
- internal/infrastructure/bufcli -> Buf CLI
- internal/infrastructure/objectstorage/s3 -> S3/MinIO
- internal/infrastructure/gitlabapi -> GitLab API
```

## Domain

`internal/domain` contains business entities, value objects, domain errors, repository ports, and vocabulary. It must not import transport, infrastructure, runtime config, logger, metrics, PostgreSQL implementations, Kafka/Sarama, or S3 SDK packages.

Important domain concepts include modules, module versions, artifacts, descriptor metadata, breaking reports, GitLab project mappings, dependency graph records, runtime services, deployments, and runtime module usages.

## Usecases

`internal/usecase` packages orchestrate application workflows:

- `registry`: module creation, version publishing, artifact retrieval, breaking checks, API tokens, and GitLab project mapping.
- `dependencygraph`: direct dependency detection from descriptor metadata.
- `runtimeinventory`: runtime service/deployment/module usage reporting and impact lookups.
- `gitlabmr`: merge request bot orchestration.
- `uiquery`: read models for the Basic Web UI.

Usecases write business state in transactions and write outbox records through the application-level outbox writer. They do not publish directly to Kafka/Sarama or import concrete PostgreSQL/S3/GitLab adapters.

## Repositories And PostgreSQL

`internal/repository/postgres` implements repository ports with PostgreSQL. Migrations live in `migrations/` and are applied by the server at startup through bootstrap wiring.

PostgreSQL stores:

- modules and versions;
- artifacts metadata;
- descriptor metadata;
- breaking reports and changes;
- GitLab project mappings;
- dependency graph edges and unresolved dependencies;
- runtime services, deployments, and module usages;
- API token hashes;
- outbox records.

## HTTP Transport

`internal/transport/http` exposes the public REST API, authentication middleware, JSON DTO mapping, health/readiness endpoints, request IDs, request logs, and Prometheus text metrics.

HTTP transport depends on usecase-facing interfaces. It does not contain business rules.

## Web Transport

`internal/transport/web` serves the read-only Basic Web UI. It uses embedded templates and static CSS, and reads through `uiquery` interfaces.

The Web UI is for local demos and trusted internal inspection. Community v1.0 does not include UI login/RBAC.

## CLI

`internal/cli` implements the `protoradar` command. The CLI talks to the server through REST API clients in `internal/cli/api`; it does not access PostgreSQL, S3, or usecases directly.

The CLI supports local login config and environment authentication through `PROTORADAR_SERVER_URL` and `PROTORADAR_TOKEN`.

## Buf Adapter

`internal/infrastructure/bufcli` shells out to the Buf CLI for:

- `buf build` during publish;
- optional `buf lint` during publish;
- `buf breaking` during breaking checks.

The server image includes the Buf CLI. Buf output is size-limited by config before being returned or stored.

## S3 And MinIO Artifact Storage

`internal/infrastructure/objectstorage/s3` stores source archives and Buf image artifacts in S3-compatible object storage. Local Docker Compose uses MinIO with path-style access.

PostgreSQL stores artifact metadata and object keys; object bodies live in S3/MinIO.

## Transactional Outbox

State-changing usecases write outbox records in the same database transaction as business state. Current event records include module creation, version publish, breaking report creation, GitLab project linking, dependency graph updates, and runtime inventory reports.

The outbox package is transport-neutral. Kafka/Sarama publishing is intentionally outside usecases and domain.

## GitLab Integration

GitLab integration has two parts:

- reusable CI templates under `examples/gitlab`;
- `protoradar gitlab mr-check`, which runs a breaking check and uses the GitLab API to comment on merge requests and set commit status.

The GitLab API adapter is infrastructure. The MR bot usecase coordinates server and GitLab clients without leaking GitLab details into domain.

## Dependency Graph

Dependency graph detection runs after publish using descriptor metadata. It detects direct dependencies from:

- proto imports;
- message field type references;
- RPC input/output type references.

Community v1.0 tracks direct edges only; it does not compute transitive closure.

## Runtime Inventory

Runtime inventory lets deployment pipelines report which module versions are running in each environment. ProtoRadar records deployments, module usages, drift status, and runtime impact for breaking reports.

Runtime reports are snapshots. Community v1.0 does not run continuous heartbeats or service discovery.
