# ProtoRadar
ProtoRadar is a self-hosted GitLab-native protobuf governance platform that helps teams detect breaking changes, understand dependency impact, review schema changes in merge requests, and track which contract versions are actually running in production.

## Roadmap

ProtoRadar is being built as a self-hosted, GitLab-native protobuf governance platform. The goal is not to create “just another protobuf registry”, but to help engineering teams answer the questions that usually appear before every risky schema change:

* Is this proto change safe to merge?
* Who will be affected by it?
* Which services depend on this contract?
* Which contract versions are actually running in production?
* Who should review or approve a breaking change?

The roadmap is intentionally split into small, demonstrable milestones. Each phase should deliver a usable piece of the product, while gradually moving ProtoRadar from a focused OSS tool to a production-ready governance platform.

---

### Phase 0 — Project Foundation

The first milestone is about defining the product clearly and setting up a maintainable technical foundation.

**Goals:**

* Define the core positioning of ProtoRadar.
* Keep the initial scope focused on protobuf/gRPC governance.
* Build a clean Go project structure suitable for long-term development.
* Prepare the repository for open-source collaboration.

**Planned work:**

* Project structure for server, CLI, domain logic, storage, integrations and deployments.
* Basic HTTP server with health, readiness and version endpoints.
* Configuration management for database, object storage, GitLab and logging.
* Structured logging with request IDs and useful operational context.
* Database migration setup.
* Docker Compose environment with the ProtoRadar server, PostgreSQL and MinIO/S3-compatible storage.
* Initial documentation, contribution notes and development guide.

**Outcome:**

A production-like Go backend skeleton that can be started locally and extended safely.

---

### Phase 1 — Core Registry MVP

The registry is the foundation of ProtoRadar. It stores protobuf modules, versions and artifacts so that teams can publish, discover and compare contract versions.

**Goals:**

* Store versioned protobuf modules.
* Provide a simple API and CLI for publishing and retrieving proto artifacts.
* Support self-hosted storage using PostgreSQL and S3-compatible object storage.

**Planned work:**

* Domain model for modules, module versions, artifacts and API tokens.
* PostgreSQL tables for modules, versions, artifacts and authentication tokens.
* Artifact storage in S3/MinIO.
* REST API for creating modules, listing modules, publishing versions and downloading artifacts.
* CLI commands:

    * `protoradar login`
    * `protoradar module create`
    * `protoradar push`
    * `protoradar pull`
    * `protoradar list`
* API token authentication for CLI and CI usage.
* Basic documentation and examples.

**Outcome:**

A working self-hosted protobuf registry that can be used from local development and CI pipelines.

---

### Phase 2 — Buf-Compatible Workflow

ProtoRadar should work with the existing protobuf ecosystem instead of replacing it. Buf compatibility is a core part of the project direction.

**Goals:**

* Support Buf-based repositories and workflows.
* Use Buf for build, lint and breaking-change analysis where possible.
* Extract useful metadata from protobuf descriptors.

**Planned work:**

* Support for `buf.yaml` and `buf.lock`.
* Ability to build and store Buf images.
* Metadata extraction for packages, services, methods, messages, enums, fields and imports.
* Compile-validity checks before publishing a module version.
* Optional lint validation depending on project configuration.
* Internal storage model for descriptor metadata.

**Outcome:**

ProtoRadar becomes a governance layer for Buf-compatible protobuf workflows rather than a competing registry implementation.

---

### Phase 3 — Breaking Change Checks

Breaking-change detection is one of the most important features of ProtoRadar. This phase introduces compatibility checks between new proto changes and previously released versions.

**Goals:**

* Compare a proposed proto change against a known released version.
* Produce clear, human-readable compatibility reports.
* Make breaking-change checks available through the CLI and API.

**Planned work:**

* CLI command for checking breaking changes:

    * `protoradar check-breaking --against latest`
    * `protoradar check-breaking --against v1.0.0`
* Integration with Buf breaking checks.
* Structured breaking-change report model.
* Storage for breaking reports in PostgreSQL.
* Human-readable output describing removed services, removed methods, changed fields, renamed messages and other incompatible changes.
* Stable exit codes for CI usage.

**Outcome:**

Developers can detect unsafe protobuf changes before they are merged or released.

---

### Phase 4 — GitLab CI Integration

ProtoRadar is designed to be GitLab-native. This phase integrates the tool into real GitLab CI workflows.

**Goals:**

* Make ProtoRadar easy to run in GitLab CI.
* Support both merge-request validation and release publishing.
* Map protobuf modules to GitLab projects.

**Planned work:**

* GitLab CI template for breaking-change checks.
* GitLab CI template for publishing proto modules on tags or releases.
* Documentation for required CI variables such as:

    * `PROTORADAR_SERVER_URL`
    * `PROTORADAR_TOKEN`
    * `PROTORADAR_MODULE`
* Module-to-GitLab project mapping.
* Example GitLab repositories showing the recommended setup.

**Outcome:**

Teams can add ProtoRadar to a GitLab project with a small CI configuration and immediately start validating protobuf changes.

---

### Phase 5 — GitLab Merge Request Bot

This is the first major “wow” feature of ProtoRadar. Instead of only failing a CI job, ProtoRadar should explain the result directly inside the merge request.

**Goals:**

* Comment on GitLab merge requests with breaking-change reports.
* Show developers what changed, why it matters and who may be affected.
* Update existing bot comments instead of creating noisy duplicates.

**Planned work:**

* GitLab API integration.
* CLI command for merge-request checks:

    * `protoradar gitlab mr-check`
* Markdown report generation for GitLab comments.
* Merge request status check integration.
* Comment sections for:

    * module name
    * base version
    * target branch or commit
    * breaking-change status
    * list of detected changes
    * potentially affected modules
* Stable CI exit codes:

    * `0` — safe change
    * `1` — breaking changes found
    * `2` — internal or tooling error

**Outcome:**

Developers get actionable protobuf governance feedback directly where code review happens.

---

### Phase 6 — Basic Web UI

The web UI makes ProtoRadar easier to understand, demo and use by teams that do not want to rely only on CLI output.

**Goals:**

* Provide a simple visual interface for modules, versions and reports.
* Make the product easy to demo locally.
* Show useful metadata extracted from proto files.

**Planned work:**

* Module list page.
* Module details page.
* Version list and version details pages.
* Breaking report details page.
* Display of packages, services, messages, methods and artifact digests.
* Basic search and filtering.
* Links between modules, versions and reports.

**Outcome:**

Users can run ProtoRadar locally, open the UI and understand the state of their protobuf contracts.

---

### Phase 7 — Dependency Graph MVP

A registry can tell where proto files are stored. ProtoRadar should also explain who depends on them.

**Goals:**

* Detect dependencies between protobuf modules.
* Show which modules may be affected by a change.
* Add impact analysis to merge-request reports.

**Planned work:**

* Import and dependency extraction from proto descriptors and Buf metadata.
* Storage model for module dependencies.
* Basic affected-consumer detection.
* UI page for dependency relationships.
* “Potentially affected modules” section in GitLab MR comments.
* Example scenario with `user-api`, `billing-api` and `notification-api`.

**Outcome:**

Before merging a schema change, teams can see which other modules may be impacted.

---

### Phase 8 — Runtime Contract Inventory

Static dependencies are useful, but production usage matters even more. Runtime inventory allows services to report which proto versions they actually use.

**Goals:**

* Track which services use which proto module versions.
* Record environment, build version and git commit for runtime reports.
* Detect stale, unknown or outdated contract usage.

**Planned work:**

* Runtime reporting API:

    * service name
    * environment
    * git commit
    * build version
    * used proto modules and versions
* CLI command:

    * `protoradar runtime report`
* Storage model for runtime services, deployments and module usage.
* UI pages for services, environments and deployed contract versions.
* Drift status:

    * up to date
    * behind latest
    * unknown version
    * deprecated version
    * potentially affected by breaking change

**Outcome:**

ProtoRadar can answer not only “what schemas exist?”, but also “what schemas are running in production?”

---

### Phase 9 — Community v1.0

The first stable OSS release should combine the registry, breaking checks, GitLab workflow, UI, dependency graph and runtime inventory into a polished self-hosted product.

**Goals:**

* Provide a complete community edition.
* Make local setup simple.
* Include documentation and examples good enough for real users.

**Planned work:**

* Stable API and CLI commands.
* Docker Compose quickstart.
* PostgreSQL and S3/MinIO support.
* GitLab CI templates.
* GitLab MR bot.
* Basic UI.
* Basic dependency graph.
* Runtime inventory MVP.
* Prometheus metrics.
* Structured logs.
* Example repositories:

    * `user-api`
    * `billing-api`
    * `notification-api`
* Documentation:

    * quickstart
    * architecture
    * CLI usage
    * GitLab CI setup
    * runtime inventory
    * deployment
    * development guide

**Outcome:**

A useful open-source protobuf governance platform that can be installed, demonstrated and adopted by small teams.

---

### Phase 10 — Governance Layer

Once the core workflow is stable, ProtoRadar can move from detection to governance.

**Goals:**

* Introduce ownership, approval and policy concepts.
* Make breaking changes controllable instead of only visible.
* Prepare the architecture for more advanced enterprise features.

**Planned work:**

* Module owners and maintainers.
* Basic policy engine.
* Approval requests for breaking changes.
* Approval decisions and audit trail.
* Rules such as:

    * breaking changes require module owner approval
    * production-used modules require affected consumer approval
    * additive changes do not require approval
* GitLab MR integration for approval status.

**Outcome:**

ProtoRadar starts enforcing contract governance policies, not just reporting compatibility problems.

---

### Phase 11 — Enterprise Architecture Boundary

ProtoRadar is planned as an open-core-friendly project. The community edition should remain useful on its own, while the architecture should allow enterprise capabilities to be added cleanly.

**Goals:**

* Keep the OSS core clean and valuable.
* Define extension points for enterprise features.
* Avoid rewriting the system later.

**Planned work:**

* Extension points for:

    * authentication providers
    * authorization policy engine
    * audit sinks
    * approval workflows
    * license and edition capabilities
* Community implementations for:

    * API token authentication
    * simple roles
    * basic audit events
    * default approval behavior
* Clear separation between community and enterprise-specific code.

**Outcome:**

The project can grow into an open-core product without compromising the open-source foundation.

---

### Phase 12 — Enterprise Features

Enterprise functionality is planned after the community edition proves the core workflow.

**Goals:**

* Support companies running private GitLab and self-hosted infrastructure.
* Add security, compliance and operational capabilities.
* Make ProtoRadar suitable for larger engineering organizations.

**Planned work:**

* OIDC login with providers such as GitLab, Keycloak and generic OIDC.
* LDAP support.
* Advanced RBAC:

    * instance admin
    * organization admin
    * module owner
    * maintainer
    * developer
    * viewer
    * service account
* GitLab group synchronization.
* Advanced audit log.
* Advanced approval workflows.
* Advanced dependency graph with ownership and environment awareness.
* Advanced runtime inventory with stale service detection and alerts.
* Helm chart.
* High-availability deployment mode.
* Backup and restore guide.
* Air-gapped installation support.
* License management.
* Enterprise documentation and support tooling.

**Outcome:**

ProtoRadar becomes suitable for organizations that need self-hosted protobuf governance with security, auditability and operational control.

---

### Long-Term Vision

The long-term vision for ProtoRadar is to become a control plane for protobuf and gRPC contract evolution in self-hosted engineering environments.

Most protobuf registries answer:

> Where are my `.proto` files?

ProtoRadar aims to answer:

> Can I safely merge this proto change, who will it affect, who must approve it, and which contract versions are actually running in production?

Future directions may include:

* Rendered protobuf documentation.
* Generated SDK artifacts.
* Webhooks and notification integrations.
* Service catalog integrations.
* Advanced graph visualization.
* Multi-format schema support after protobuf support is mature.
* Optional hosted version after the self-hosted product is stable.

The priority is to stay focused: build a narrow, reliable and production-minded protobuf governance platform before expanding into broader schema management.
