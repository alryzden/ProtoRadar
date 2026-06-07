# Event-flow rules

Canonical home: this file owns outgoing event/command flow and transactional outbox ownership. Runtime publisher lifecycle safety belongs in `docs/development/production-safety.md`; package dependency boundaries belong in `docs/development/architecture.md`.

## Outgoing events and commands

Usecases never publish to Kafka directly.

Correct flow:

```text
usecase
  -> DB transaction
  -> business state changes
  -> outbox.Writer.Create(outbox.Record)
  -> commit
  -> outbox publisher
  -> transport-neutral Dispatcher
  -> optional concrete transport adapter if one is explicitly wired
```

## Package ownership

- domain: event type vocabulary and business reasons.
- integration/protoradarevents: ProtoRadar event payload DTOs and constructors.
- integration/protoradarcommands: ProtoRadar command DTOs and constructors.
- integration/<service>commands: command DTOs and constructors for external services.
- outbox: durable record and publisher loop.
- infrastructure/kafka: topic routing, Kafka keys, headers, Sarama publishing only if a Kafka adapter exists.

## Adding a new outgoing ProtoRadar event

1. Add event type/reason vocabulary to domain only if it is real business vocabulary.
2. Add payload DTO and constructor to `internal/integration/protoradarevents`.
3. Constructor must return `outbox.Record`.
4. Write the record from usecase inside the transaction.
5. Add routing/key/header tests in the concrete transport adapter if one exists.
6. Add usecase tests for transactional outbox behavior.
7. Do not import Kafka/Sarama in usecase/domain.

## Adding a new outgoing command

1. Put the command contract in `internal/integration/protoradarcommands` if it belongs to ProtoRadar.
2. Put the command contract in `internal/integration/<service>commands` if it targets an external service.
3. Constructor returns `outbox.Record`.
4. Use a stable dedup key/idempotency key.
5. Route it in the concrete transport adapter if one exists.
6. Keep commands separate from ProtoRadar domain events.
