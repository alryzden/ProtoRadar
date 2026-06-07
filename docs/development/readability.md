# Readability and Maintainability Rules

Canonical home: this file owns readability, maintainability, lint interpretation, acceptable complexity, and generated or mechanical-looking code smells. Production lifecycle and resource safety rules live in `docs/development/production-safety.md`. Test isolation and performance rules live in `docs/development/testing.md`.

Use this file together with:

- `docs/development/architecture.md` for package boundaries;
- `docs/development/configuration.md` for config ownership;
- `docs/development/event-flow.md` for transactional outbox/event flow;
- `docs/development/production-safety.md` for lifecycle, shutdown, context, transaction, resource, timeout, secret, and observability safety;
- `docs/development/testing.md` for test-specific constraints.

## 1. Core Principle

Code is not complete just because it compiles and tests pass.

It should also be easy for a human maintainer to scan, debug, explain, and safely change.

Prefer boring, explicit, bounded code over clever abstractions.

## 2. Rule Ownership

Do not duplicate detailed rules across files.

When a topic has a canonical home:

- update the canonical file first;
- keep summaries in other files short;
- avoid adding a second full version of the same checklist;
- mark older verification reports as historical if they no longer describe the current state.

## 3. Readability And Human Maintainability

Watch for:

- very large files with unrelated concerns;
- long functions doing many things;
- deeply nested control flow;
- boolean parameters with unclear meaning;
- optional nil dependencies causing hidden behavior;
- generic names like `manager`, `processor`, `handler`, `helper`, `data`, or `result`;
- abstractions with only one unclear implementation;
- huge DTO/mapping files;
- large test files with unrelated test families;
- fake implementations more complex than production logic;
- repeated boilerplate that can drift.

Preferred cleanup style:

- split files by feature or area;
- add short section comments when they improve navigation;
- extract domain-specific helpers;
- keep direct control flow where it is clearer;
- keep boundary DTOs explicit;
- keep tests grouped by behavior.

Do not introduce a framework, reflection, registry table, generic abstraction, or option-builder pattern just to reduce line count.

## 4. Helper And Abstraction Rules

Add an abstraction only when it:

- clarifies ownership;
- hides real third-party or infrastructure detail;
- reduces meaningful duplication;
- makes lifecycle boundaries safer;
- creates a real test seam without package-level mutable globals;
- matches an established local pattern.

Avoid one-use helpers that only wrap:

- a direct constructor call;
- simple struct copying;
- a single field assignment;
- a local validation `if` that is clearer inline.

Helper names should explain why the helper exists, not merely restate what it does.

## 5. Generated or Mechanical-Looking Code Smells

Review for:

- overly generic helper names;
- comments that repeat the code;
- excessive wrappers around simple calls;
- too many option structs/builders for simple construction;
- repeated validation without a clear boundary reason;
- copy-pasted error handling with tiny message differences;
- broad "just in case" nil checks;
- fake future extensibility;
- mechanical table tests that hide intent;
- large files created by appending new code instead of reorganizing nearby code.

Fix by making intent visible, not by adding more abstraction.

## 6. Acceptable Complexity

Do not remove useful architecture just because it adds navigation.

The following can be acceptable if they protect real boundaries:

- repository ports;
- storage ports;
- auth provider interfaces;
- authorization policy interfaces;
- audit sinks;
- outbox/event dispatchers;
- transaction managers;
- DTOs at transport boundaries;
- import-boundary tests;
- integration fakes.

Flag them only if their local implementation is unnecessarily hard to read.

Some explicit validation, routing, CLI parsing, and orchestration functions may have higher cyclomatic complexity because the branching is domain-significant. Prefer targeted refactors over splitting behavior into vague helpers.

## 7. Lint And Static Analysis Policy

Use tools to find risks, but do not blindly apply every suggestion.

Recommended tools for Go projects:

```bash
go test ./...
go vet ./...
go mod tidy -diff
staticcheck ./...
golangci-lint run
gocyclo -over 15 internal cmd
```

High-value findings:

- unchecked important errors;
- missing timeouts;
- dead code;
- deprecated APIs;
- nil error returns;
- shadowing that affects readability;
- high cyclomatic complexity in changing code;
- body/rows/resource leaks;
- security-sensitive warnings.

Noisy or mechanical findings must be reviewed before enforcement. Do not enable broad style checks in CI unless the project has agreed that the check is worth the churn.

Examples of findings that often need project-specific policy:

- parameter type combining;
- range value copy in DTO/view mapping loops;
- overly strict function length;
- aggressive line wrapping;
- style-only import grouping beyond standard Go grouping.

Do not globally disable safety linters only to make CI green. Prefer:

- fixing real bug-risk findings;
- disabling specific noisy checkers;
- narrow path/text exclusions with comments;
- documenting intentionally accepted complexity.

## 8. Documentation And Verification Reports

Verification READMEs are useful, but they can become confusing when later work supersedes them.

When creating or updating a report:

- state whether it is current or historical;
- link or name the newer report if it supersedes older results;
- avoid claiming a known historical failure is still current;
- record skipped or unavailable tools honestly;
- do not overclaim that implementation is complete when only audit/design work was done.

If an older report contains stale command results, do not rewrite history unless the task is specifically to update that report. Prefer adding a newer final report that clearly records the current state.

## 9. Test Readability

Detailed test safety and performance policy lives in `docs/development/testing.md`.

For readability, avoid:

- single huge test files covering many unrelated areas;
- test setup that hides the important assertion;
- generic `panic("not implemented")`;
- broad fakes with silent zero-value returns;
- brittle full-output snapshots when targeted assertions are enough;
- source-level grep tests without clear failure messages.

Prefer:

- test files grouped by route/page/command/usecase family;
- focused helpers with domain-specific names;
- method-specific unexpected-call failures;
- explicit assertions for behavior being protected;
- table tests only when cases are easy to read.

Unexpected fake calls must fail loudly and specifically.

## 10. Useful Searches

Use these when reviewing readability or maintainability:

```bash
rg -n "TODO|FIXME|HACK|XXX" internal cmd docs scripts
rg -n "interface \{" internal
rg -n "Manager|Processor|Helper|Util|Common" internal cmd
rg -n "panic\\(\"not implemented\"\\)" internal cmd
rg -n "nolint|//nolint|lint:ignore" internal cmd
```

Use the broader lifecycle/resource searches from `docs/development/production-safety.md` when reviewing production safety.

## 11. Severity Model

Use this severity model for readability and maintainability findings:

### Critical

Code structure is likely to cause corrupted state, security mistakes, or unsafe release behavior because maintainers cannot reason about it safely.

### High

Code is hard to modify safely, hides important behavior, or is likely to create production bugs during normal evolution.

### Medium

Meaningful readability issue with limited blast radius, or a complexity hotspot that should be addressed before adding more behavior.

### Low

Minor naming, local organization, comment, test readability, or style issue.

### Informational

Reviewed and acceptable by design.

## 12. Reporting Requirements

When reporting a readability or maintainability problem, include:

- stable ID if producing an audit;
- severity;
- file and line;
- pattern;
- evidence;
- why it is hard to read or risky to change;
- why tests may not catch it;
- suggested fix direction;
- what not to change;
- whether it blocks release or the next phase;
- whether it is confirmed or needs verification.

Do not mark acceptable architecture as a bug merely because it adds navigation.

## 13. Final Rule

Prefer code that is:

- explicit;
- boring;
- bounded;
- observable;
- testable;
- easy to delete;
- easy to debug;
- easy for a human to explain.

Avoid code that is:

- clever;
- magical;
- over-abstracted;
- silently best-effort;
- hard to trace;
- generated-looking;
- correct only because many distant pieces happen to align.
