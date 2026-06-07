# Development Documentation

These files define ProtoRadar's development rules for contributors, maintainers, and automated coding tools. Each policy has one canonical home; other documents may summarize it but should not duplicate the full rulebook.

- `architecture.md` owns package boundaries, layer responsibilities, and HTTP authorization boundaries.
- `event-flow.md` owns transactional outbox behavior and event/command ownership.
- `configuration.md` owns raw config, defaults, validation, and typed runtime mapping.
- `go-style.md` owns Go style, helper design, imports, local errors, and test seams.
- `production-safety.md` owns lifecycle, shutdown, cleanup, context, resource, transaction, secret, CLI/CI, and observability safety.
- `readability.md` owns readability, maintainability, lint interpretation, and acceptable complexity.
- `testing.md` owns test isolation, Testcontainers, TestMain, cleanup, performance, external services, test data, and test helper behavior.
- `review-checklist.md` owns the final checklist before returning changes.
