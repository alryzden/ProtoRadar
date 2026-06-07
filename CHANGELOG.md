# Changelog

## v1.0.0 - Community v1.0

### Added

- Self-hosted protobuf registry for modules, versions, and artifacts.
- Buf-compatible publish workflow with server-side `buf build`, optional `buf lint`, descriptor metadata extraction, source archive storage, and Buf image storage.
- Server-side breaking change checks against published baselines.
- Governance layer with module owners/maintainers, deterministic built-in policies, approval requests, approval requirements, approval decisions, and governance audit trail.
- GitLab CI templates for breaking checks, publishing, MR bot workflows, and runtime inventory reporting.
- GitLab merge request bot command for comments, commit status, affected modules, runtime impact context, and governance status.
- Read-only Basic Web UI for modules, versions, metadata, breaking reports, dependency graph, runtime inventory, module owners, approval status, approval request details, and governance audit trail.
- Direct dependency graph detection from proto imports, field type references, and RPC method references.
- Runtime inventory reporting for deployed service/module versions, drift status, and breaking report runtime impact.
- Module version deprecation metadata, Community deprecate workflow, and runtime `deprecated_version` drift reporting for services using deprecated versions.
- Prometheus text metrics at `/metrics`.
- Health and readiness endpoints at `/healthz` and `/readyz`.
- Structured request logs with request IDs and token redaction.
- Docker Compose local stack with PostgreSQL, MinIO, bucket initialization, and example `.env` config.
- Dockerfile `cli` target, Makefile build/smoke targets, and GitLab CI documentation for using a local or privately published `protoradar` CLI image.
- Community examples under `examples/repos` and local demo/smoke scripts.
- Stable CLI exit code convention for success, breaking findings, and operational errors.
- Governance REST and CLI workflows for owner management and approval decisions.
- Phase 11 Enterprise Architecture Boundary with Community identity/auth provider, authorization, audit sink, approval workflow, capability checker, edition metadata, `GET /api/v1/edition`, `protoradar edition`, and `/ui/about`.

### Release Notes

- `protoradar version` reports version, commit, and build date.
- Makefile and Dockerfile builds can inject release metadata through ldflags-compatible variables.
- Local HTTP request bodies are bounded by `PROTORADAR_SERVER_MAX_REQUEST_BODY_BYTES`; compressed source archives are bounded separately by `PROTORADAR_REGISTRY_MAX_ARTIFACT_SIZE_BYTES`.

### Known Limitations

- Dependency graph is direct-only; no transitive traversal yet.
- Runtime reports are deployment snapshots, not continuous heartbeats or service discovery.
- Basic Web UI has no built-in login/RBAC; use a trusted network or authentication proxy.
- Governance actors are derived from authenticated API-token principals by default; OIDC, LDAP, advanced RBAC, GitLab group sync, license management, and custom policy DSL are not implemented.
- Waiver workflows are not implemented.
- Kubernetes operator workflows are not implemented.
- Breaking diagnostic parsing is best-effort and depends on Buf output.
- Generated SDK workflows and client usage tracking are not implemented.
- Metrics are always exposed in Community v1.0; there is no separate metrics config flag.
- Pre-v1.0 development schemas may require fresh install or manual migration.
