# Test Safety and Performance Rules

Canonical home: this file owns test safety, integration-test isolation, Testcontainers, TestMain, cleanup, performance, external services, test data, and test helper behavior. Production lifecycle safety belongs in `docs/development/production-safety.md`; general readability guidance belongs in `docs/development/readability.md`.

This file defines general engineering rules for writing fast, reliable, self-contained, and maintainable automated tests.

The rules are intentionally project-agnostic. They are suitable for Go backend, CLI, API, worker, and infrastructure-heavy projects, especially projects that use databases, object storage, queues, external APIs, or containers in integration tests.

Use these rules together with the project's architecture, code-style, bug-risk, and review-checklist rules.

---

## 1. Core Principles

Tests should be:

- self-contained;
- deterministic;
- fast enough to run frequently;
- isolated from each other;
- explicit about external dependencies;
- safe to run locally and in CI;
- clear when skipped;
- easy to debug when they fail.

A test is not good enough just because it passes. It must also be understandable, maintainable, and practical to run.

---

## 2. Prefer Self-Contained Integration Tests

Integration tests should not require a manually prepared local service when a containerized dependency can be started automatically.

Avoid requiring developers to manually set variables such as:

```bash
TEST_DATABASE_URL=...
TEST_REDIS_URL=...
TEST_MINIO_ENDPOINT=...
```

Prefer automatic setup through Testcontainers or an equivalent container test framework.

Manual external dependency variables may be acceptable only when:

- the dependency cannot reasonably run in a container;
- the test is explicitly marked as an external/system test;
- the docs explain how to run it;
- normal integration tests do not depend on it.

---

## 3. Container-Based Test Dependencies

Use containers for integration dependencies such as:

- PostgreSQL;
- MySQL/MariaDB;
- Redis;
- MinIO/S3-compatible storage;
- Kafka/Redpanda;
- NATS;
- Elasticsearch/OpenSearch;
- local cloud-service emulators.

Container tests must:

- use pinned image tags;
- use non-deprecated container-library APIs;
- use bounded setup contexts/timeouts;
- use bounded cleanup contexts/timeouts;
- register cleanup reliably;
- avoid printing credentials or full connection strings;
- skip clearly when Docker/container runtime is unavailable;
- never require a developer to manually create schemas, users, buckets, queues, or topics.

Example preferred image style:

```text
postgres:16-alpine
redis:7-alpine
minio/minio:<pinned-version>
```

Avoid floating tags such as `latest` in tests unless the project deliberately wants to test against moving targets.

---

## 4. Fast Integration Test Strategy

Starting one container per test is often correct but can become too slow.

Prefer this pattern when safe:

```text
one shared container per package/test process
+ one isolated database/schema/bucket/namespace per test
+ per-test cleanup
```

For PostgreSQL-like databases, a strong pattern is:

```text
one shared PostgreSQL container per package
+ one isolated schema per test
+ per-test search_path
+ migrations applied per schema
+ DROP SCHEMA ... CASCADE during cleanup
```

This preserves isolation while avoiding repeated container startup cost.

Use one container per test only when:

- database-level state cannot be isolated safely;
- tests mutate global database objects;
- tests require different server-level settings;
- isolation is more important than runtime;
- the test count is small enough that performance is acceptable.

---

## 5. Database Test Isolation

Database integration tests must not depend on test order or shared table state.

Acceptable isolation strategies:

1. One container per test.
2. One database per test inside a shared container.
3. One schema per test inside a shared database.
4. One transaction per test with rollback, only if all code under test can safely share the same transaction.

For backend repository tests, schema-per-test is often a good default.

If using schema-per-test:

- generate safe unique schema names;
- never use user input in schema names;
- quote identifiers safely;
- set the connection `search_path` for every test connection;
- verify `current_schema()` in helper tests;
- apply migrations inside the test schema;
- verify application tables are not accidentally created in `public`;
- clean up with `DROP SCHEMA IF EXISTS <schema> CASCADE`.

---

## 6. Migrations in Tests

Integration tests should use the same migration path that production startup uses, unless the test is intentionally unit-level.

Do not maintain a separate test schema setup that drifts from production migrations.

Rules:

- run real migrations for repository/integration tests;
- verify migrations are idempotent;
- verify migration state is isolated per test database/schema;
- avoid hand-created test tables that bypass migration coverage;
- keep migration tooling contained in infrastructure/test layers;
- do not leak migration library types into domain/usecase packages.

If using Goose, Flyway, Liquibase, Atlas, golang-migrate, or another migration tool, tests should exercise that same runner where practical.

---

## 7. Cleanup and Resource Lifecycle

Every test resource must have a cleanup path.

This includes:

- containers;
- networks;
- volumes;
- database pools;
- schemas/databases;
- object-storage buckets;
- queues/topics;
- temp files/directories;
- goroutines;
- HTTP test servers;
- background workers.

Use the test framework's cleanup mechanism, for example Go's `t.Cleanup`, whenever possible.

Cleanup must be bounded. Do not let failed cleanup hang forever.

Recommended pattern:

```go
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

if err := cleanup(ctx); err != nil {
    t.Errorf("cleanup test resource: %v", err)
}
```

Do not call `os.Exit`, `log.Fatal`, or `panic` in test setup helpers. Use `t.Fatal`, `t.Fatalf`, `t.Skip`, or returned errors.

`os.Exit` in `TestMain` is acceptable only if all cleanup has already completed.

---

## 8. Skip Behavior

When Docker or another required runtime is unavailable, integration tests may skip.

Skips must be clear and actionable.

Good skip message:

```text
skipping PostgreSQL integration test: Docker/Testcontainers provider is unavailable: <reason>
```

Bad skip message:

```text
skip
```

Do not silently pass integration tests without running meaningful assertions.

If a container starts successfully, real application/test errors must fail the test, not skip.

---

## 9. Performance Rules

Measure test performance before and after test infrastructure changes.

Track:

- package test runtime;
- full suite runtime;
- slowest tests;
- container startup count;
- whether results are cached or non-cached.

Use non-cached commands for timing comparisons:

```bash
go test ./internal/repository/postgres -count=1 -v
```

Do not optimize by weakening tests.

Good optimizations:

- shared container per package;
- isolated schema/database per test;
- avoiding repeated image pulls;
- reusing an admin pool safely;
- reducing unnecessary sleeps;
- using readiness checks instead of fixed waits;
- moving expensive setup to package scope while preserving per-test isolation.

Bad optimizations:

- sharing mutable table state;
- relying on test order;
- skipping migrations;
- removing assertions;
- making tests pass against mocks when integration behavior matters;
- hiding failures as skips;
- enabling parallelism without proving isolation.

---

## 10. Parallelism

Do not add `t.Parallel()` or equivalent just to make slow integration tests look faster.

Parallelism is allowed only when:

- each test has independent state;
- shared helpers are concurrency-safe;
- cleanup cannot delete another test's resources;
- test data does not depend on timing/order;
- the dependency can handle concurrent connections/load;
- the test suite has been run repeatedly to detect flakiness.

Before enabling parallelism, add isolation proof tests.

Useful checks:

- two tests/schemas cannot see each other's data;
- migration state is per test schema/database;
- no test writes to shared global tables;
- no package-level mutable state is used unsafely.

If performance is already good after shared containers, prefer serial tests over risky parallelism.

---

## 11. TestMain Safety

If using Go's `TestMain`, be careful with cleanup and `os.Exit`.

Safe pattern:

```go
func TestMain(m *testing.M) {
    code := runTests(m)
    os.Exit(code)
}

func runTests(m *testing.M) int {
    cleanup, err := setup()
    if err != nil {
        // Store unavailable reason so tests can skip, or return non-zero for required setup.
    }

    code := m.Run()

    if cleanup != nil {
        cleanup()
    }

    return code
}
```

Rules:

- cleanup must run before `os.Exit`;
- do not use `log.Fatal` in `TestMain`;
- do not call `os.Exit` from helper functions;
- keep setup errors clear;
- do not print secrets.

---

## 12. Secrets and Logs in Tests

Tests must not print:

- database passwords;
- full DSNs;
- API tokens;
- Authorization headers;
- object-storage secrets;
- full environment dumps.

Container libraries may log image/container lifecycle metadata. Project helpers should avoid adding secret-bearing logs on top.

If a connection string is needed for debugging, redact credentials first.

---

## 13. Test Helper Design

Good test helpers are:

- small;
- explicit;
- named after what they provide;
- cleanup-safe;
- not overly magical;
- easy to use consistently.

Prefer names like:

```go
newTestPostgresPool(t)
newMigratedTestPostgresPool(t)
newIsolatedTestSchema(t)
newTestObjectBucket(t)
```

Avoid helpers that hide too much behavior behind vague names like:

```go
setup(t)
prepare(t)
makeEnv(t)
createEverything(t)
```

A helper that starts a real dependency should make that clear in its name or package context.

Fake or generated-looking test helpers must fail loudly on unexpected calls.

Allowed in tests when intentional:

- method-specific `panic("fakeRepo.Method was called unexpectedly")`;
- `context.Background()` for local unit tests;
- `t.Fatal`/`t.Fatalf`;
- source-level guardrail tests with clear failure messages.

Avoid:

- generic `panic("not implemented")`;
- broad fake methods returning zero values silently;
- copying unsafe production patterns into helpers without a test-only reason.

---

## 14. Unit, Integration, and System Test Boundaries

Keep test levels clear.

Unit tests:

- no real external dependencies;
- fast;
- use fakes/stubs;
- target business logic and edge cases.

Integration tests:

- use real infrastructure dependencies in containers;
- test repositories/adapters/transports;
- use migrations and real clients where practical;
- remain self-contained.

System/e2e tests:

- may run multiple services together;
- may be slower;
- should be explicitly labeled or documented;
- should not block normal fast development loops unless intentionally required.

Do not replace integration tests with unit tests just to improve speed.

---

## 15. External Service Tests

For external APIs such as GitLab, GitHub, Stripe, Slack, cloud APIs, or identity providers:

Prefer:

- fake HTTP servers;
- contract-style adapter tests;
- recorded minimal fixtures only if safe;
- container/emulator if available and reliable.

Avoid normal tests depending on live external services.

If live external tests are needed, mark them separately and require explicit opt-in.

---

## 16. Object Storage and Queue Tests

For object storage:

- use MinIO or another S3-compatible container when possible;
- create one bucket/prefix per test;
- delete buckets/objects during cleanup;
- avoid sharing object prefixes across tests;
- do not print access keys or secret keys.

For queues/brokers:

- use one namespace/topic/stream/consumer group per test where possible;
- clean up topics/streams where supported;
- use bounded waits;
- avoid sleeps as synchronization;
- prefer readiness checks and polling with deadlines.

---

## 17. Avoid Flaky Timing

Avoid:

```go
time.Sleep(5 * time.Second)
```

Prefer:

- readiness checks;
- polling with deadline;
- context timeouts;
- eventually-style helpers with clear failure messages.

Polling helper requirements:

- bounded total timeout;
- small interval;
- clear error on timeout;
- no busy loop.

---

## 18. Assertions and Failure Messages

Test failures should explain:

- what was expected;
- what was observed;
- which resource/test case failed;
- relevant IDs/names without secrets.

For complex struct/list comparisons, consider using `go-cmp` or a similar diff tool if it improves clarity.

Do not add assertion libraries just to reduce a few lines. Use them where they make failures clearer.

---

## 19. Test Data

Test data should be:

- minimal;
- named clearly;
- unique when needed;
- generated through helpers when repetitive;
- not dependent on wall-clock time unless explicitly testing time behavior.

Avoid hidden coupling through reused global fixtures.

If test data must be shared, keep it immutable.

---

## 20. CI Requirements

If integration tests use containers, CI must provide:

- Docker or compatible container runtime;
- permission to access the runtime;
- enough CPU/memory;
- network access for image pulls or pre-pulled images;
- appropriate caching strategy where possible.

CI docs should state:

- which tests require Docker;
- how tests behave when Docker is unavailable;
- expected approximate runtime;
- how to run only fast/unit tests if supported;
- how to run integration tests explicitly if separated.

---

## 21. Required Review Checklist for Test Changes

Before finishing any test-related change, check:

1. Does this test require a manually prepared external service?
2. Can the dependency be started automatically in a container?
3. Are container image tags pinned?
4. Are setup and cleanup contexts bounded?
5. Are all resources cleaned up with `t.Cleanup` or equivalent?
6. Are credentials/DSNs/tokens redacted from logs?
7. Do tests skip clearly when Docker/runtime is unavailable?
8. Does the test fail, not skip, after the dependency starts and real behavior is wrong?
9. Is per-test state isolated?
10. Are migrations or setup paths the same as production where relevant?
11. Are tests fast enough for frequent local/CI runs?
12. Is a shared container safe and useful?
13. Is parallelism proven safe before being enabled?
14. Did performance improve without reducing assertions?
15. Did production code avoid test-only dependencies?
16. Did architecture/import-boundary tests still pass?
17. Are docs updated for local/CI test execution?

---

## 22. Useful Commands

Common Go checks:

```bash
gofmt -l $(find . -name '*.go' -not -path './.git/*')
go test ./...
go vet ./...
go mod tidy -diff
staticcheck ./...
golangci-lint run
```

Non-cached timing:

```bash
go test ./... -count=1
```

Package-specific timing:

```bash
go test ./internal/repository/postgres -count=1 -v
```

Search for environment-dependent tests:

```bash
rg -n "TEST_.*URL|TEST_DATABASE_URL|TEST_REDIS_URL|TEST_.*ENDPOINT" .
```

Search for container usage:

```bash
rg -n "testcontainers|RunContainer|postgres:|redis:|minio" .
```

Search for unsafe test process exits:

```bash
rg -n "os\.Exit|log\.Fatal|panic\(" internal cmd
```

Search for parallel tests:

```bash
rg -n "t\.Parallel\(" .
```

Search for database schema/search path handling:

```bash
rg -n "search_path|current_schema|CREATE SCHEMA|DROP SCHEMA|goose_db_version|schema_migrations" .
```

---

## 23. Reporting Requirements

When changing test infrastructure, create a verification note or PR summary that includes:

- previous problem;
- new test architecture;
- dependency/container strategy;
- isolation strategy;
- cleanup strategy;
- skip behavior;
- commands run;
- whether Docker-backed tests actually ran or skipped;
- before/after timings if performance was a goal;
- remaining limitations;
- whether production code imports test-only dependencies.

For performance work, include exact timing commands and whether results were cached or non-cached.

---

## 24. Final Rule

Do not trade correctness for speed.

The best integration tests are:

```text
self-contained + isolated + fast enough + honest about dependencies
```

A fast test suite that hides failures is worse than a slow test suite.

A slow but correct test suite should be optimized through better test infrastructure, not by deleting coverage.
