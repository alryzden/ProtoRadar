# Development Instructions for ProtoRadar

## Purpose

This file is the short entrypoint for contributors, maintainers, and automated coding tools. Detailed policies live in `docs/development/`.

## Required Reading

- `docs/development/architecture.md`
- `docs/development/event-flow.md`
- `docs/development/configuration.md`
- `docs/development/go-style.md`
- `docs/development/production-safety.md`
- `docs/development/readability.md`
- `docs/development/testing.md`
- `docs/development/review-checklist.md`

## Core Invariants

- Preserve clean architecture boundaries between domain, usecase, outbox, events, integration contracts, infrastructure adapters, repositories, transport, bootstrap, and runtime helpers.
- Usecases do not publish external events or commands directly. State changes and `outbox.Record` creation must happen transactionally inside the same database transaction.
- Kafka/Sarama routing, publishing, keys, headers, and transport details stay outside domain and usecase packages.
- Config parsing, defaults, validation, and typed runtime mapping stay in `internal/config`; bootstrap only adapts typed config into concrete constructors.
- `internal/app/bootstrap` remains composition-only and must not grow business rules.
- The CLI remains REST-client oriented except for documented narrow exceptions.
- The Web UI must not import repositories directly.
- Migrations use Goose and the `goose_db_version` model. For active feature work, update an existing related uncommitted migration pair instead of creating fix-up migration chains.
- PostgreSQL tests must be self-contained and Testcontainers-based.
- Phase 12 or private/enterprise implementation code does not belong in the public OSS repository unless explicitly approved.

## Standard Verification

```sh
gofmt -l $(find . -name '*.go' -not -path './.git/*')
go test ./...
go vet ./...
go mod tidy -diff
staticcheck ./...
golangci-lint run
make check-gitlab-templates
```

## Reporting

Temporary local review reports should not be committed.
