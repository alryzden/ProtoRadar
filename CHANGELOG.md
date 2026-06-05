# Changelog

## v1.0.0 - Community v1.0

### Added

- Self-hosted protobuf registry for modules, versions, and artifacts.
- Buf-compatible publish workflow with server-side `buf build`, optional `buf lint`, descriptor metadata extraction, source archive storage, and Buf image storage.
- Server-side breaking change checks against published baselines.
- GitLab CI templates for breaking checks, publishing, MR bot workflows, and runtime inventory reporting.
- GitLab merge request bot command for comments, commit status, affected modules, and runtime impact context.
- Read-only Basic Web UI for modules, versions, metadata, breaking reports, dependency graph, and runtime inventory.
- Direct dependency graph detection from proto imports, field type references, and RPC method references.
- Runtime inventory reporting for deployed service/module versions, drift status, and breaking report runtime impact.
- Prometheus text metrics at `/metrics`.
- Health and readiness endpoints at `/healthz` and `/readyz`.
- Structured request logs with request IDs and token redaction.
- Docker Compose local stack with PostgreSQL, MinIO, bucket initialization, and example `.env` config.
- Community examples under `examples/repos` and local demo/smoke scripts.
- Stable CLI exit code convention for success, breaking findings, and operational errors.

### Release Notes

- `protoradar version` reports version, commit, and build date.
- Makefile and Dockerfile builds can inject release metadata through ldflags-compatible variables.
- Local HTTP request bodies are bounded by `PROTORADAR_MAX_REQUEST_BODY_BYTES`; compressed source archives are bounded separately by `PROTORADAR_REGISTRY_MAX_ARTIFACT_SIZE_BYTES`.

### Known Limitations

- Dependency graph is direct-only; no transitive traversal yet.
- Runtime reports are deployment snapshots, not continuous heartbeats or service discovery.
- Basic Web UI has no built-in login/RBAC; use a trusted network or authentication proxy.
- Approval, waiver, and policy workflow features are not implemented.
- OAuth/OIDC and Kubernetes operator workflows are not implemented.
- Breaking diagnostic parsing is best-effort and depends on Buf output.
- Generated SDK workflows and client usage tracking are not implemented.
- Metrics are always exposed in Community v1.0; there is no separate metrics config flag.
- Pre-v1.0 development schemas may require fresh install or manual migration.
