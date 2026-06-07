# Config rules

Canonical home: this file owns config defaults, raw YAML/env parsing, validation, typed runtime mapping, and bootstrap/config boundaries. Other rule files may summarize this policy but should not duplicate the full details.

## Desired style

Config should be readable, grouped by component, and easy to extend.

`internal/config` owns:

- raw YAML/env config structs;
- defaults;
- env overrides;
- validation;
- typed `RuntimeConfig`;
- duration parsing.

`internal/app/bootstrap` owns only mapping typed config to concrete infrastructure constructors.

## Rules

- Do not parse raw duration strings in bootstrap.
- Do not duplicate config structs in bootstrap.
- Do not silently ignore invalid production-critical values.
- Use small validation helpers for common checks:
  - required string;
  - required string group;
  - positive int;
  - non-negative int;
  - positive duration;
  - duration ordering.
- Avoid generic reflection-heavy validation.
- Avoid generics for validation helpers unless the project already uses them.
- Avoid maps for ordered validation when deterministic error order matters.
- Prefer slices of small structs for grouped validations.

## Good validation shape

```go
func (c Config) Validate() error {
    if err := c.validateDatabase(); err != nil {
        return err
    }
    if err := c.validateKafka(); err != nil {
        return err
    }
    if err := c.validateOutboxPublisher(); err != nil {
        return err
    }
    return nil
}
```

## Bad signs

- huge Validate() with 100 repeated if err := ...;
- bootstrap has `time.ParseDuration`;
- defaults split between YAML, config and bootstrap;
- one-use helper functions that hide simple code instead of clarifying boundaries.
