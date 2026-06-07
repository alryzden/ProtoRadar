# Architecture

ProtoRadar follows clean architecture with explicit package boundaries. Business state changes happen in usecases, persistence is behind repository ports, and external systems are wired in bootstrap.

Architecture/import-boundary tests in `internal/archtest` enforce the most important package rules automatically. They run as part of `go test ./...` and are intended to keep the Community core clean while preserving extension points for future private/open-core work.

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
internal/usecase/governance
internal/identity  internal/authorization  internal/audit  internal/edition
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

Important domain concepts include modules, module versions, artifacts, descriptor metadata, breaking reports, GitLab project mappings, dependency graph records, runtime services, deployments, runtime module usages, module owners, approval requests, approval requirements, approval decisions, and governance audit events.

## Usecases

`internal/usecase` packages orchestrate application workflows:

- `registry`: module creation, version publishing, artifact retrieval, breaking checks, API tokens, and GitLab project mapping.
- `dependencygraph`: direct dependency detection from descriptor metadata.
- `runtimeinventory`: runtime service/deployment/module usage reporting and impact lookups.
- `governance`: module ownership, built-in policy evaluation, approval request creation, approval decisions, audit writes, and transactional outbox records.
- `gitlabmr`: merge request bot orchestration.
- `uiquery`: read models for the Basic Web UI.

Phase 11 extension boundaries live in application-level packages:

- `identity`: principal model, request context helpers, `AuthProvider`, and Community API token provider.
- `authorization`: action/resource model, `Authorizer`, and Community authorizer.
- `audit`: product audit event model, `AuditSink`, and Community audit sink.
- `edition`: capability model, `CapabilityChecker`, Community capability checker, and edition model.

Usecases write business state in transactions and write outbox records through the application-level outbox writer. They do not publish directly to Kafka/Sarama or import concrete PostgreSQL/S3/GitLab adapters.

## Repositories And PostgreSQL

`internal/repository/postgres` implements repository ports with PostgreSQL. Migrations live in `migrations/` and are applied by Goose at server startup through bootstrap wiring. Goose records migration state in `goose_db_version`; the old custom `schema_migrations` state table is not used.

PostgreSQL stores:

- modules and versions;
- artifacts metadata;
- descriptor metadata;
- breaking reports and changes;
- GitLab project mappings;
- dependency graph edges and unresolved dependencies;
- runtime services, deployments, and module usages;
- module owners, approval requests, approval requirements, approval decisions, and governance audit events;
- API token hashes;
- outbox records.

Module version rows include deprecation metadata: `deprecated_at`, `deprecated_by`, and `deprecation_reason`. Repositories persist and load this metadata; domain/usecase logic interprets `deprecated_at` as the deprecation marker. Deprecation is metadata only and does not remove artifact records or object storage data.

## HTTP Transport

`internal/transport/http` exposes the REST API, authentication middleware, JSON DTO mapping, health/readiness endpoints, request IDs, request logs, and Prometheus text metrics.

HTTP server lifecycle settings, including read/write/idle timeouts and maximum header bytes, are parsed and validated in `internal/config`. Bootstrap wires the typed runtime values into `net/http.Server` and does not parse raw config values.

HTTP transport depends on usecase-facing interfaces and Phase 11 boundaries. It authenticates bearer tokens through `identity.AuthProvider`, stores the returned `identity.Principal` in request context, authorizes normal API routes through `authorization.Authorizer`, and serves authenticated edition metadata at `GET /api/v1/edition`. It does not contain business rules.

HTTP route classes are explicit:

- public operational routes: `GET /healthz`, `GET /readyz`, and `GET /metrics`;
- bootstrap-only route: `POST /api/v1/tokens`, which uses the bootstrap bearer token to create API tokens;
- protected API routes: all other `/api/v1` routes, each registered with an authorization action and resource.

The authorization model is generic. Actions represent operations such as module reads, version publishing, GitLab mapping management, breaking report reads, runtime inventory reporting, governance owner management, approval decisions, governance audit reads, and edition reads. Resources identify targets such as modules, module versions, breaking reports, runtime services/environments, approval requests, governance owner records, and edition metadata. Handlers should receive requests only after authentication and authorization have completed.

HTTP authorization tests are part of this boundary. Every normal authenticated API route must have coverage proving missing authentication returns `401 unauthorized`, a denying `Authorizer` returns `403 forbidden`, an allowing `Authorizer` reaches the handler path, and the expected action/resource pair is passed. Public and bootstrap-only routes must be listed explicitly and tested as not using normal `Authorizer` denial.

The route guardrail tests live in `internal/transport/http/server_test.go`. `authenticatedRouteExpectations` is the protected-route action/resource table. `routeClassificationExpectations` classifies public, bootstrap-only, and protected routes, and checks registered routes in `server.go` for accidental direct `requireBearer` use. Some allow-path unit tests intentionally stop at handler validation or not-found responses because their purpose is to prove that authorization allowed the handler to run without requiring PostgreSQL, S3, Buf, GitLab, Kafka, or network access.

## Web Transport

`internal/transport/web` serves the read-only Basic Web UI. It uses embedded templates and static CSS, and reads through `uiquery` interfaces.

The Web UI is for local demos and trusted internal inspection. It displays module owners, approval status, approval request details, and governance audit trail. Community v1.0 does not include UI login/RBAC or state-changing approval forms.

`/ui/about` displays Community edition, build metadata, and enabled/unavailable capabilities through the `uiquery` service.

## CLI

`internal/cli` implements the `protoradar` command. Normal CLI commands talk to the server through REST API clients in `internal/cli/api`; they do not access PostgreSQL, S3, HTTP server handlers, or server-side repositories directly.

The current exception is `protoradar gitlab mr-check`: it performs client-side GitLab API calls and reuses the transport-neutral `internal/usecase/gitlabmr` orchestration package locally. The import-boundary tests allow only that exact CLI exception. Do not add broader CLI-to-usecase or CLI-to-infrastructure imports without revisiting the architecture.

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

State-changing usecases write outbox records in the same database transaction as business state. Current event records include module creation, version publish, breaking report creation, GitLab project linking, dependency graph updates, runtime inventory reports, module owner changes, approval request creation, approval decisions, and approval request status changes.

The invariant is:

- the usecase changes domain state inside a database transaction;
- the usecase writes an `outbox.Record` through `outbox.Writer` inside the same transaction;
- the usecase commits;
- an outbox publisher, outside domain/usecase business logic, claims and dispatches records.

Usecases never publish integration events directly.

Outbox records move through a durable lifecycle:

- `pending`: written transactionally and ready when `available_at` is due;
- `processing`: claimed by a publisher worker with `claimed_at` and `claim_expires_at`;
- `published`: dispatched successfully and not claimable again;
- `failed`: dispatch failed and the record is retryable after `available_at`;
- `dead`: dispatch failed at the configured maximum attempts.

Claiming is implemented in the PostgreSQL repository with row locking and `FOR UPDATE SKIP LOCKED`, so multiple publisher loops can avoid claiming the same row. Failed records use exponential backoff up to the configured maximum backoff. Expired `processing` claims are reclaimable when their lease has elapsed.

The outbox package is transport-neutral. Community wiring uses a local `LoggingDispatcher`: it logs event metadata such as record ID, event type, aggregate, dedup key, attempts, and payload size, but it does not deliver to Kafka/Sarama, webhooks, or any external event bus. When the Community publisher is enabled, successful local logging dispatch marks records `published`. When it is disabled, records remain durable and `pending` until a publisher is enabled or another dispatcher is wired.

Future private/open-core extensions can provide another `outbox.Dispatcher` implementation from their own composition root. Dispatchers must honor `Dispatch(ctx, record)` cancellation promptly and should configure their own external network/client timeouts. Server shutdown cancels the publisher and waits only for a bounded interval before reporting a close error, so a dispatcher that ignores cancellation can leave claimed records to recover through normal lease expiry instead of marking unfinished work as published. OSS core does not depend on enterprise code, and Kafka/Sarama delivery remains out of scope unless a real adapter is explicitly added later.

## Import-Boundary Tests

The `internal/archtest` package contains source/import checks for the root architecture rules. Run them with the normal test suite:

```sh
go test ./...
```

These tests enforce direct production imports and a small AST scan for config parsing calls. They check that:

- `internal/domain` does not import infrastructure, transport, config, app/bootstrap, repository/postgres, SQL/pgx, S3/MinIO SDKs, Buf adapters, GitLab concrete clients, Kafka/Sarama, or enterprise package paths;
- `internal/usecase` does not import concrete infrastructure, repository/postgres, HTTP/Web transport, CLI code, S3/Buf/GitLab concrete adapters, Kafka/Sarama, topic routing, SQL/pgx, or enterprise package paths;
- `internal/transport/http` and `internal/transport/web` do not import repository/postgres or low-level persistence/event/storage adapters directly;
- `internal/cli` remains REST-client oriented, with only the documented GitLab MR local-orchestration exception;
- `internal/outbox` remains transport-neutral and does not import Kafka/Sarama, transports, CLI, repository/postgres, or enterprise package paths;
- `internal/identity` and `internal/authorization` remain transport-free and do not import repository/postgres, CLI, OIDC/LDAP/RBAC concrete implementations, or enterprise package paths;
- OSS production packages under `internal` and `cmd` do not import enterprise package paths;
- raw application config parsing stays centralized in `internal/config` where the tests can detect it, while CLI-only environment handling remains a narrow exception.

Import tests are guardrails, not proof of correct architecture. They do not prove that transport handlers are free of business logic, that bootstrap is wiring-only, or that every config misuse is impossible. Code review still has to check those constraints.

## GitLab Integration

GitLab integration has two parts:

- reusable CI templates under `examples/gitlab`;
- `protoradar gitlab mr-check`, which runs a breaking check and uses the GitLab API to comment on merge requests and set commit status.

The GitLab API adapter is infrastructure. The MR bot usecase coordinates server, governance, and GitLab clients without leaking GitLab details into domain. The MR Markdown renderer only renders governance state; it does not compute policy.

## Governance

Governance is implemented in `internal/usecase/governance` and persisted through domain repository ports. The policy evaluator is deterministic and infrastructure-free. It operates on breaking report summaries, module owner data, affected modules, runtime impact, and typed governance config.

Phase 10 built-in rules are:

- passed/non-breaking reports do not require approval;
- breaking reports require changed module owner approval;
- production-used affected modules require affected consumer approval;
- missing owners create visible pending requirements.

Governance audit events are product audit records stored in PostgreSQL. They are not outbox records. Outbox records are integration events used for durable external publication and remain transport-neutral.

## Enterprise Boundary

Phase 11 prepares ProtoRadar for downstream open-core builds without adding enterprise implementations to OSS. Community implementations are wired in bootstrap:

- API token authentication through `APITokenAuthProvider`;
- simple authorization through `CommunityAuthorizer`;
- basic audit through `CommunityAuditSink`;
- default approval behavior through `DefaultApprovalWorkflow`;
- capability/edition reporting through `CommunityCapabilityChecker`.

Enterprise packages may depend on OSS interfaces, but OSS core must not depend on enterprise packages. See [Enterprise Boundary](enterprise-boundary.md).

The Authorizer denial coverage is an enterprise-readiness guardrail, not an OSS RBAC implementation. It proves the HTTP transport invokes the replaceable authorization boundary for normal API routes. `CommunityAuthorizer` remains permissive for Community product actions by design, and strict RBAC policy remains future downstream/private work.

## Dependency Graph

Dependency graph detection runs after publish using descriptor metadata. It detects direct dependencies from:

- proto imports;
- message field type references;
- RPC input/output type references.

Community v1.0 tracks direct edges only; it does not compute transitive closure.

## Runtime Inventory

Runtime inventory lets deployment pipelines report which module versions are running in each environment. ProtoRadar records deployments, module usages, drift status, and runtime impact for breaking reports.

Runtime reports are snapshots. Community v1.0 does not run continuous heartbeats or service discovery.

Runtime drift is calculated in the runtime inventory usecase from registry module-version data. Transport, Web UI, CLI, and GitLab render the server-calculated status and do not recalculate drift. The normal runtime drift precedence is `unknown_version`, `deprecated_version`, `behind_latest`, then `up_to_date`. Breaking report runtime impact uses separate `potentially_affected_by_breaking_change` context and can carry the stored runtime drift status alongside it.
