# Code style rules

Canonical home: this file owns Go style, helper design, import style, test seams, and local error style. Config ownership details live in `docs/development/configuration.md`; production lifecycle safety lives in `docs/development/production-safety.md`; readability/lint policy lives in `docs/development/readability.md`.

## General style

- Prefer simple, direct code over excessive abstraction.
- Do not create helper functions that are used only once unless they hide real infrastructure details, reduce meaningful duplication, or make lifecycle ownership clearer.
- Do not create package-level variables for dependency injection in production code, such as `newKafkaProducer = ...`, unless there is a strong reason and it is documented.
- Prefer explicit dependencies through structs/constructors/bootstrap wiring over mutable package-level function variables.
- Avoid aliases for imports unless required to resolve a name conflict or to improve clarity.
- Do not use aliases like `kafkainfra`, `postgresrepo`, `protoradarv1` randomly; use them only when they improve readability or avoid collision.
- Keep functions small, but do not split trivial one-line construction into extra functions if it makes navigation harder.

## Imports

- Imports must be grouped in this order:
    1. standard library;
    2. external dependencies;
    3. internal project packages.
- Run `gofmt`/`goimports` after changes.
- Avoid unnecessary import aliases.
- Do not import infrastructure packages from domain/usecase/outbox/events.

## Helpers

- Helper names must describe why the helper exists, not just what it wraps.
- One-use helpers are acceptable only for:
    - lifecycle ownership;
    - hiding third-party API details;
    - making tests simpler without package-level mutable globals;
    - reducing repeated non-trivial code.
- One-use helpers are usually not acceptable for:
    - simple struct mapping;
    - direct constructor call;
    - `metrics.NewRegistry()`;
    - `logger.NewWithConfig(...)`;
    - `pgplatform.Config{...}` if used once.

## Test seams

- Do not introduce production package-level variables only to replace dependencies in tests.
- Prefer interfaces, explicit constructor dependencies, fake implementations, or test-specific builders.
- If a production seam is required, keep it local to bootstrap/composition root and document why.

## Errors

- Domain sentinel errors belong only to domain/business errors.
- Startup/wiring/config validation errors should stay in config/bootstrap/usecase constructor packages.
- Do not add sentinel errors unless callers/tests need `errors.Is`.
- Error messages should include the config path or dependency name.

## Config

- Config style and ownership are defined in `docs/development/configuration.md`.
- Do not duplicate config structs or parsing rules here.
- When touching config code, preserve deterministic validation errors and keep raw parsing in `internal/config`.
