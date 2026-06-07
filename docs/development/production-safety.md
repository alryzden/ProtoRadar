# Production Safety Rules

Canonical home: this file owns production bug-risk rules for lifecycle, shutdown, cleanup, context propagation, transactions, resources, secrets, CLI/CI safety, and observability. Readability-only guidance belongs in `docs/development/readability.md`; test-specific safety/performance guidance belongs in `docs/development/testing.md`.

This file defines project-wide engineering rules for preventing subtle lifecycle, cleanup, shutdown, concurrency, context, transaction, resource, and observability bugs.

These rules are permanent contributor guidance for Go backend, CLI, worker, integration, and service projects.

Use these rules together with the project's architecture, code style, configuration, testing, and review rules.

A change is not complete just because it compiles and passes tests. It must also preserve cleanup correctness, bounded shutdown, context propagation, transaction safety, resource closure, timeout behavior, secret redaction, and observability for best-effort failures.

---

## 1. Process Exit and Fatal Calls

### Rule

Do not call `os.Exit`, `log.Fatal`, `log.Fatalf`, or equivalent fatal process-termination functions after resources requiring cleanup have been initialized.

This is especially important after any of the following has happened:

- `defer Close()`
- `defer Shutdown()`
- `defer cancel()`
- database pool/client creation
- HTTP server creation
- background worker start
- queue/outbox/event publisher start
- file/temp directory creation
- metrics/logging/tracing setup
- external client setup that requires cleanup

In Go, `os.Exit` skips deferred functions. This can bypass application-level cleanup.

### Required pattern

Prefer a small `main` function that exits only after the application runner has returned:

```go
func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	// Load config.
	// Initialize resources.
	// Run application.
	// Close resources explicitly or through defers.
	// Return final exit code only after cleanup has completed.
}
```

or:

```go
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

In the second form, `run()` must not call `os.Exit` internally.

### Forbidden pattern

```go
defer server.Close()

if err := server.HTTPServer.Shutdown(ctx); err != nil {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1) // forbidden: skips server.Close()
}
```

### Main package guidance

`os.Exit` is acceptable only at the outermost process boundary, after cleanup has completed.

Internal packages, libraries, usecases, repositories, clients, workers, handlers, and adapters must return errors. They must not terminate the process.

---

## 2. Shutdown and Cleanup Must Be Explicit

Every component that starts or owns resources must have a clear shutdown path.

This includes:

- HTTP servers
- database pools
- background goroutines
- queue consumers/producers
- outbox/event publishers
- dispatchers
- workers
- tickers
- timers
- file handles
- object storage readers/writers
- temporary files/directories
- telemetry exporters
- external API clients when they own resources

If a component starts a goroutine, the owning component must define how it is stopped and waited for.

### Server shutdown order

Server shutdown should generally:

1. stop accepting new work;
2. cancel background contexts;
3. wait for workers with a bounded wait when appropriate;
4. close database/object/client resources;
5. flush logs/metrics/traces where applicable;
6. return cleanup errors to the caller;
7. avoid process exit before cleanup completes.

### Bounded shutdown

Do not wait forever for background workers that may depend on external systems.

Prefer:

```go
cancel()

select {
case <-done:
	return nil
case <-time.After(timeout):
	return fmt.Errorf("component did not stop within %s", timeout)
}
```

If a timeout is configurable, parsing and validation must live in the project's configuration layer, not in bootstrap/wiring code.

---

## 3. Goroutine and Channel Lifecycle

Every production goroutine must have:

- a cancellation path;
- a clear owner;
- a bounded or documented lifetime;
- a way to report errors if errors matter;
- a non-blocking shutdown path.

Avoid:

- goroutines that ignore `ctx.Done()`;
- goroutines sending to unbuffered channels without a guaranteed receiver;
- goroutines that can leak on early return;
- closing a channel from multiple places;
- `select { default: ... }` busy loops;
- `sync.WaitGroup.Add` inside goroutines;
- missing `WaitGroup.Done`;
- missing `ticker.Stop`;
- missing `timer.Stop`.

### Error channels

If a goroutine sends one terminal error, use a buffered channel:

```go
errCh := make(chan error, 1)
go func() {
	errCh <- run()
}()
```

This avoids blocking if another `select` branch wins.

---

## 4. Context Propagation

Use caller-provided context in request, usecase, repository, storage, external-client, command-execution, and worker flows.

Do not use `context.Background()` inside usecases, repositories, HTTP handlers, storage adapters, API clients, command runners, or long-running operations when a caller context is available.

### Acceptable `context.Background()` usage

`context.Background()` is acceptable:

- at the outermost CLI/server root;
- when creating a shutdown timeout that must not inherit an already-canceled signal context;
- in tests;
- in clearly documented independent background setup.

### External calls

External calls must have timeouts through at least one of:

- request context deadline;
- client timeout;
- command timeout;
- typed config timeout.

This applies to:

- HTTP clients;
- third-party API calls;
- command execution;
- object storage operations where supported;
- database operations;
- queue/event systems;
- long-running CI/client commands.

---

## 5. HTTP Server and Client Lifecycle

### HTTP servers

Production HTTP servers must set explicit lifecycle limits:

- `ReadHeaderTimeout`
- `ReadTimeout`
- `WriteTimeout`
- `IdleTimeout`
- `MaxHeaderBytes`

These values must come from typed config with safe defaults and validation.

Bootstrap/wiring code may pass typed values into `http.Server`, but must not parse raw config.

### HTTP clients

Do not use timeout-less `http.DefaultClient` for production clients.

If a constructor accepts a nil `*http.Client`, it must replace it with a finite-timeout client.

Custom injected clients must be preserved in tests.

### Streaming responses

If headers may already be sent, do not attempt to write a second JSON error response after a streaming failure.

Instead:

- capture the copy/read error;
- close the body;
- log the failure with bounded metadata;
- increment a metric if available;
- let the client detect truncated streams through connection/checksum behavior.

---

## 6. Transactions and Database Resources

Transaction handling must be explicit and safe.

Use the project's transaction manager/unit-of-work pattern where one exists.

Do not scatter writes across multiple operations if atomicity is required.

### Commit and rollback

- Never ignore `Commit` errors.
- Rollback errors should not hide the original error.
- If rollback fails, join or log it according to project style.
- Do not double-commit or double-rollback.
- Do not write audit/outbox/event records outside the state-changing transaction when the event must reflect committed state.

### Rows

For SQL rows:

```go
defer rows.Close()

for rows.Next() {
	// scan
}

if err := rows.Err(); err != nil {
	return err
}
```

Always check final `rows.Err()`.

---

## 7. Event, Outbox, Queue, and Worker Safety

Usecases may write durable event/outbox records transactionally.

Usecases must not publish directly to Kafka, NATS, RabbitMQ, SQS, webhooks, email, Slack, Teams, SIEM, or any external bus unless the architecture explicitly allows direct publishing.

Prefer:

- transactional state change;
- durable outbox/event record;
- separate publisher/dispatcher;
- bounded retry/dead-letter behavior;
- transport-neutral interfaces.

### Dispatcher contract

Every dispatcher implementation must:

- honor `ctx` cancellation promptly;
- use finite network/client timeouts;
- avoid blocking forever;
- avoid logging secrets or full large payloads;
- return errors instead of panicking or terminating the process.

Server shutdown must not hang indefinitely because of a dispatcher.

---

## 8. File, Archive, and Object Resource Cleanup

### Files

Every opened file must be closed.

Do not defer `file.Close()` inside loops or `filepath.WalkDir` callbacks if many files can be opened.

Prefer helper functions:

```go
func addFile(...) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	copyErr := copyFile(f)
	closeErr := f.Close()

	return errors.Join(copyErr, closeErr)
}
```

### Temporary files and directories

Temporary files/directories must be removed unless intentionally retained.

Use safe paths and avoid deleting computed paths without validation.

### Archives

Archive extraction must reject:

- absolute paths;
- `..` traversal;
- unclean paths;
- backslash traversal where relevant;
- symlinks unless explicitly supported and safe;
- hardlinks unless explicitly supported and safe;
- unsupported entry types;
- unbounded decompressed sizes.

### Object storage

Object readers/writers must be closed.

If object cleanup is best-effort, failures must be observable through logs or metrics.

---

## 9. Error Handling and Observability

Do not silently ignore errors unless the behavior is explicitly best-effort and observable.

Suspicious patterns include:

```go
_ = something.Close()
_ = tx.Rollback()
_ = store.Delete(...)
_ = token.MarkUsed(...)
_, _ = io.Copy(...)
```

These are acceptable only when:

1. the error truly cannot affect correctness;
2. the primary error is preserved;
3. the best-effort nature is documented;
4. important failures are logged or metriced where useful.

### Best-effort operations

Best-effort operations must be explicit.

Examples:

- cleanup after failed publish/upload;
- metadata update after successful authentication;
- rollback cleanup after primary failure;
- close after already-failed write;
- telemetry/log flush during shutdown.

Best-effort does not mean invisible.

Prefer:

- bounded structured log;
- metric counter;
- returned joined error if it does not hide the primary error.

---

## 10. Token, Secret, and Identity Safety

Never log or print:

- raw API tokens;
- OAuth/OIDC/JWT tokens;
- Git provider tokens;
- Authorization headers;
- cookies/session tokens;
- full environment dumps;
- request bodies containing secrets;
- unredacted external API error bodies.

CI templates and scripts must not use:

- `set -x`
- `printenv`
- `env`
- `echo $TOKEN`
- `echo $API_TOKEN`
- `echo $GITLAB_TOKEN`
- token CLI flags when env-based auth is available

### Error bodies

External API error bodies must be bounded and redacted before logging or returning.

### Actor and identity

Audit/security actors should come from authenticated identity/principal context, not caller-supplied request fields.

Caller-supplied actor overrides must be explicitly enabled, documented, and audited if supported.

---

## 11. Configuration Ownership

Raw config parsing belongs in the project's configuration layer.

Do not parse environment variables, durations, sizes, booleans, or server/client timeouts in bootstrap, usecase, repository, transport, domain, or business logic packages.

Bootstrap may consume typed runtime config only.

Every new config field must include:

- default;
- parsing;
- validation;
- docs;
- tests;
- runtime mapping test where applicable.

---

## 12. CLI and CI Safety

### CLI

CLI tools must not import server repositories or server usecases unless the project explicitly allows a narrow exception.

Network operations in CLI tools must have finite default timeouts.

CLI commands must not print secrets.

### CI templates

CI templates must:

- validate required variables without printing values;
- avoid token CLI args when env-based auth is available;
- avoid full environment dumps;
- avoid `set -x`;
- respect documented exit codes.

---

## 13. Production vs Test Code

These rules apply to production code.

Test code may use:

- `panic`;
- `context.Background`;
- `t.Fatal`;
- deferred cleanup patterns;
- in-memory clients;
- fake goroutines;

but tests should not normalize unsafe production patterns.

If a risky pattern appears in test code only, mark it as acceptable only if it cannot leak into production behavior.

---

## 14. Required Review Checklist for Every Code Change

Before finishing any code change, check:

1. Does this change introduce `os.Exit`, `log.Fatal`, or `panic` in production code?
2. Are all resources closed on every path?
3. Can any defer be skipped by process exit?
4. Are background goroutines cancellable and waited for?
5. Can shutdown hang forever?
6. Are HTTP server/client timeouts explicit?
7. Are contexts propagated correctly?
8. Are transaction commit/rollback and `rows.Err()` handled?
9. Are audit/outbox/event writes transactional where required?
10. Are file/archive/object readers closed promptly?
11. Are cleanup failures observable?
12. Are ignored errors intentional and documented?
13. Are secrets redacted from logs/errors/CI output?
14. Does the change preserve architecture/import boundaries?
15. Does the change require new config, docs, or tests?
16. Does the change need a race test, shutdown test, timeout test, or lifecycle test?

---

## 15. Commands and Searches to Run When Relevant

For lifecycle-sensitive changes, run or consider:

```bash
go test ./...
go vet ./...
go mod tidy -diff
gofmt -l $(find . -name '*.go' -not -path './.git/*')
```

If the project has CI/template checks, run them too.

Useful searches:

```bash
rg -n "os\.Exit|log\.Fatal|log\.Fatalf|panic\(" internal cmd
rg -n "defer .*Close\(|defer .*Shutdown\(|defer .*Stop\(|defer .*cancel\(" internal cmd
rg -n "_ = .*Close\(|_ = .*Rollback\(|_ = .*Commit\(|_ = .*Shutdown\(" internal cmd
rg -n "_ = .*Delete|_ = .*MarkUsed|_, _ = io\.Copy" internal cmd
rg -n "context\.Background\(\)|context\.TODO\(\)" internal cmd
rg -n "go func\(|go [a-zA-Z0-9_\.]+\(" internal cmd
rg -n "time\.NewTicker|time\.Tick\(|time\.After\(|time\.NewTimer" internal cmd
rg -n "http\.DefaultClient|http\.Client\{|ListenAndServe|Shutdown\(|Server\{" internal cmd
rg -n "rows\.Next\(|rows\.Err\(|rows\.Close\(" internal
rg -n "BeginTx|Commit\(|Rollback\(" internal
rg -n "io\.Copy|ReadAll|LimitReader|MaxBytesReader" internal cmd
rg -n "os\.Open|os\.Create|os\.MkdirTemp|os\.CreateTemp|RemoveAll" internal cmd scripts
rg -n "Authorization|TOKEN|API_TOKEN|GITLAB_TOKEN|GITHUB_TOKEN|printenv|set -x" .github .gitlab examples scripts docs internal cmd 2>/dev/null || true
```

Optional if available:

```bash
staticcheck ./...
golangci-lint run
go test -race ./...
```

Use judgment. Do not run expensive or destructive commands unless the project expects them.

---

## 16. Severity Model

Use this severity model when reporting subtle bug risks.

### Critical

Can cause:

- data loss;
- corrupted business state;
- security leak;
- guaranteed cleanup skip in normal production error path;
- unrecoverable process behavior in normal operation.

### High

Can cause:

- skipped important cleanup;
- shutdown bugs;
- leaked workers/resources;
- stuck production server/client lifecycle;
- swallowed important operational errors;
- hard-to-debug production incidents.

### Medium

Suspicious pattern with limited blast radius, or a bug that requires unusual conditions but is plausible.

### Low

Cleanup, observability, or metadata issue unlikely to break correctness but worth improving.

### Informational

Reviewed pattern that is acceptable by design.

---

## 17. Reporting Requirements

When a possible bug-risk is found, report:

- file;
- line;
- pattern;
- severity;
- why it is risky;
- why tests may not catch it;
- suggested fix direction;
- whether it blocks release, production-hardening, or current work;
- whether it is confirmed or needs verification.

Do not hide uncertain findings. Mark them clearly as `Needs verification`.

Do not mark acceptable patterns as bugs if evidence shows they are safe.

---

## 18. Common Hotspots

Pay special attention to:

- `cmd/` entrypoints;
- bootstrap/composition root;
- HTTP server setup;
- HTTP handlers that stream data;
- API clients;
- database repositories;
- transaction helpers;
- background workers;
- queue/event/outbox publishers;
- file/archive/object storage code;
- CLI upload/download/archive code;
- CI scripts and templates;
- auth/token/identity code;
- telemetry/logging setup;
- migration runners.

These areas are more likely to contain lifecycle, cleanup, context, timeout, or observability mistakes.

---

## 19. Final Rule

A change is not complete just because it compiles and passes tests.

A change is complete only when it also preserves:

- cleanup correctness;
- bounded shutdown;
- context propagation;
- transaction safety;
- resource closure;
- timeout behavior;
- secret redaction;
- event/outbox/worker lifecycle rules;
- architecture boundaries;
- observability for best-effort failures.
