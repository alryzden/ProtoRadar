# Review Checklist

Canonical home: this file is the short final checklist. Detailed rules live in the specialized files listed in `AGENTS.md`; do not expand this file into a second full rulebook.

Before finishing a task, verify:

## Imports

- `internal/domain` does not import infrastructure/transport/config/logger/postgres/sarama/outbox lifecycle.
- `internal/usecase` does not import Kafka/Sarama/Postgres implementations/gRPC transport.
- `internal/outbox` does not import Kafka/Sarama/Postgres implementations.
- `internal/events` does not import Kafka/Sarama/Kafka infrastructure.
- `cmd/*` do not manually build low-level infra if bootstrap already owns it.
- `internal/archtest` import-boundary tests pass, and any new exception is narrow, documented, and not a broad layer allowlist.

## Event-flow

- Usecase writes outbox inside the same DB transaction as business state.
- Kafka publishing is not done in usecase.
- New outgoing event/command has tests for payload, dedup key, routing, key and headers.
- Consumers remain idempotent.

## Config

- New config fields have defaults, env/yaml binding, validation and runtime mapping.
- Invalid durations/topics/required settings fail fast.
- Bootstrap does not parse raw string durations.
- Raw server env parsing and duration parsing stay in `internal/config`; CLI-only env handling remains a narrow exception.

## Tests

Run:

```sh
gofmt -w <changed go files>
go test ./...
```

Update README/docs only if behavior or structure changed.

## HTTP Authorization

- New normal `/api/v1` routes use the HTTP protected wrapper with an explicit `authorization.Action` and `authorization.Resource`.
- No normal authenticated route uses direct `requireBearer` without `Authorizer`; only documented bootstrap-token flows may bypass normal bearer authorization.
- New protected routes have tests for missing/invalid auth, denying `Authorizer`, allowing `Authorizer`, and exact action/resource recording.
- Public and bootstrap-only routes are explicitly classified in HTTP route guardrail tests and documented when caller-visible behavior changes.
- `routeClassificationExpectations` and `authenticatedRouteExpectations` stay complete for current `server.go` route registrations.

## Lint and reports

- Do not enable noisy style-only linters without project-specific justification.
- Do not globally disable safety linters to make CI green.
- If a lint finding is intentionally accepted, prefer a narrow config exclusion with a comment.
- New or updated verification READMEs clearly state whether results are current, partial, skipped, or historical.
