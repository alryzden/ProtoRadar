# Architecture rules

Canonical home: this file owns package/layer dependency rules. Lifecycle safety belongs in `docs/development/production-safety.md`; readability/lint policy belongs in `docs/development/readability.md`; final task checks belong in `docs/development/review-checklist.md`.

## Layers

### Domain

Allowed:

- entities;
- value objects;
- business constants;
- domain errors;
- repository ports;
- business vocabulary.

Forbidden:

- Kafka/Sarama;
- PostgreSQL implementation details;
- gRPC/protobuf mapping;
- runtime config;
- logger/metrics;
- outbox publishing lifecycle;
- JSON integration payload DTOs.

### Usecase

Allowed:

- business orchestration;
- transaction boundaries;
- calls to repository ports;
- calls to engines;
- writing `outbox.Record` through `outbox.Writer`.

Forbidden:

- direct Kafka publishing;
- Sarama imports;
- SQL implementation details;
- gRPC/protobuf types;
- topic resolver usage;
- infrastructure constructors.

### Integration

Contains external message contracts:

- outgoing ProtoRadar events;
- outgoing ProtoRadar commands;
- outgoing commands for external services;
- JSON envelope/schema/payload DTOs;
- constructors returning `outbox.Record`.

Integration packages may depend on domain vocabulary and `outbox.Record`.

### Outbox

Contains:

- durable outbox record;
- writer port;
- publisher repository port;
- transport-neutral message builder/publisher ports;
- publisher loop.

Outbox must not import Kafka/Sarama/Postgres implementations.

### Events

Contains:

- normalized incoming message model;
- dispatcher;
- handlers;
- error classifier;
- ACK/retry/DLQ decisions;
- DLQ publisher port.

Events must not import Kafka/Sarama infrastructure.

### Infrastructure

Contains concrete adapters:

- Kafka/Sarama producer;
- Kafka outbox message builder;
- Kafka topic resolver;
- Kafka DLQ adapter;
- Sarama consumer adapter.

Infrastructure may import application ports and domain/integration contracts.

### Bootstrap

Composition root only:

- create logger;
- create metrics;
- create DB pool;
- create repositories;
- create engines/usecases;
- create Kafka adapters;
- assemble per-binary dependency graphs.

Bootstrap must not contain business rules.

### HTTP Authorization Boundary

Normal authenticated `/api/v1` routes must go through the HTTP protected wrapper with an explicit `authorization.Action` and `authorization.Resource`.

Allowed exceptions must be narrow and documented:

- public operational endpoints such as health/readiness/metrics;
- bootstrap-token flows that intentionally run before normal bearer authorization;
- compatibility endpoints explicitly covered by route classification tests.

New protected routes need tests for:

- missing or invalid authentication;
- denying authorizer;
- allowing authorizer;
- exact action/resource recording;
- route classification coverage.
