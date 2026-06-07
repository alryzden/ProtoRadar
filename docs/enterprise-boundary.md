# Enterprise Architecture Boundary

## Overview

ProtoRadar uses an open-core-friendly architecture. Community Edition remains useful on its own: it includes the registry, Buf workflow, breaking checks, GitLab CI/MR workflows, dependency graph, runtime inventory, governance approvals, basic audit, metrics, logs, Docker Compose, CLI, API, and Web UI.

Phase 11 adds explicit extension points so future downstream or enterprise builds can replace selected behavior at bootstrap/wiring time. The OSS core does not depend on enterprise-specific packages, and OSS interfaces do not mention enterprise concrete types.

The repository includes automated import-boundary tests in `internal/archtest` to guard this separation. They run with `go test ./...` and fail if OSS production packages import enterprise package paths or pull concrete infrastructure into inner layers.

## Community Edition Responsibilities

Community Edition currently wires these implementations:

- `internal/identity.APITokenAuthProvider` authenticates API bearer tokens and returns an API-token principal.
- `internal/authorization.CommunityAuthorizer` keeps current Community behavior: authenticated principals can use Community product features, while admin/enterprise-only actions stay unavailable.
- `internal/audit.CommunityAuditSink` records product audit through the existing governance audit storage.
- `internal/usecase/governance.DefaultApprovalWorkflow` preserves Phase 10 approval behavior.
- `internal/edition.CommunityCapabilityChecker` reports Community capabilities enabled and represented Enterprise capabilities disabled.

## Extension Points

The Phase 11 boundaries are:

- `identity.AuthProvider`: authenticates an `AuthRequest` and returns a `Principal`.
- `authorization.Authorizer`: authorizes a principal, action, and resource.
- `audit.AuditSink`: records bounded product audit events.
- `governance.ApprovalWorkflow`: creates approval requests, reads approval status, and records approval decisions.
- `edition.CapabilityChecker`: checks and lists available capabilities.
- `edition.Edition`: describes edition name, build metadata, and capability statuses.

These contracts are application/product boundaries. They do not include HTTP request types, CLI types, PostgreSQL implementations, Kafka/Sarama, license managers, OIDC clients, LDAP clients, or enterprise concrete types.

## Authentication Providers

Community Edition uses API token authentication. The HTTP middleware extracts the bearer token, calls `AuthProvider`, and stores the returned `Principal` in request context. The raw token is used only for authentication and is not used as the principal subject. API-token principals use token metadata such as token name or token ID.

Future downstream builds, including Phase 12/private enterprise work, may replace the API-token principal with an OIDC-, LDAP-, GitLab OIDC-, Keycloak-, or generic OIDC-backed user principal. No enterprise authentication provider exists in the OSS repository.

## Authorization

Community Edition uses simple authorization that preserves existing behavior. Authenticated principals can use Community product actions such as module reads/writes, version publishing, GitLab mapping reads/management, breaking checks and reports, dependency graph reads, runtime inventory, governance owner reads/management, approval request reads/creation, approval decisions, governance audit reads, and edition reads. Admin/enterprise actions are not enabled.

All normal authenticated HTTP API routes pass through `Authorizer`. Public routes are limited to operational endpoints (`/healthz`, `/readyz`, `/metrics`), and API token creation remains bootstrap-token-only at `POST /api/v1/tokens`. This keeps the authorization boundary replaceable: downstream builds can enforce stricter action/resource policy without changing individual HTTP handlers.

The action/resource contract is intentionally generic:

- actions describe operations;
- resources describe targets such as modules, module versions, breaking reports, runtime inventory objects, approval requests, governance owner records, and edition metadata;
- handlers should not bypass `Authorizer` for normal API routes.

HTTP tests cover the replaceable boundary explicitly. For protected routes they prove missing authentication fails before authorization, denying authorizers produce forbidden responses, allowing authorizers reach handler behavior, and the expected action/resource pair is passed. Public and bootstrap-only routes are classified separately and tested as not using normal `Authorizer` denial.

Future downstream builds may provide advanced RBAC, organization/team policies, scoped service accounts, or group-based rules. The OSS package does not include RBAC tables or an enterprise policy engine.

## Audit Sinks

Community Edition has basic audit through `CommunityAuditSink`, backed by the existing governance audit repository/table. Governance owner changes, approval request creation, approval decisions, and approval status changes route through the audit sink while staying transaction-compatible with the existing usecases. Governance usecases consume an effective actor/trusted principal boundary from the server-side HTTP/auth layer. In OSS, that actor is derived from authenticated API-token principals by default; legacy caller-supplied actor override is a disabled-by-default Community compatibility setting.

Future downstream builds may provide advanced audit sinks such as SIEM integrations, immutable logs, file sinks, webhooks, or separate audit storage. Audit is separate from the transactional outbox; outbox records remain integration events for durable external publication.

## Approval Workflows

Community Edition uses `DefaultApprovalWorkflow`, which wraps the Phase 10 approval service and preserves these rules:

- passed/non-breaking reports create or return `not_required`;
- breaking reports require changed module owner approval;
- production-used affected modules require affected consumer approval;
- missing owners remain visible as pending requirements;
- approval/rejection writes audit and outbox records transactionally.

Future downstream builds may provide advanced workflows, multi-step approvals, escalation, SLA handling, or custom routing. No advanced workflow engine or custom policy DSL exists in OSS.

## Capabilities And Editions

`CommunityCapabilityChecker` enables Community capabilities:

- `registry`
- `buf_workflow`
- `breaking_checks`
- `gitlab_ci`
- `gitlab_mr_bot`
- `web_ui`
- `dependency_graph`
- `runtime_inventory`
- `community_governance`
- `api_token_auth`
- `basic_audit`
- `prometheus_metrics`
- `structured_logs`
- `docker_compose_quickstart`

Enterprise capabilities are represented but disabled:

- `oidc_auth`
- `ldap_auth`
- `advanced_rbac`
- `gitlab_group_sync`
- `advanced_audit`
- `advanced_approval_workflows`
- `enterprise_dependency_graph`
- `runtime_alerts`
- `helm_ha`
- `air_gapped`
- `license_management`

The server exposes this through authenticated `GET /api/v1/edition`. The CLI exposes it through:

```sh
protoradar edition
```

The Web UI exposes it at `/ui/about`.

## Downstream And Enterprise Wiring

Downstream builds can provide alternative implementations in their composition root. Enterprise packages may depend on OSS core packages and interfaces, but OSS core packages must not import enterprise packages.

Keep these rules:

- Bootstrap wires implementations; it does not contain business policy.
- Interfaces stay concrete-type neutral.
- Config changes belong in `internal/config`.
- Domain does not import infrastructure, transport, config, logger, repository/postgres, S3 SDKs, Buf CLI adapters, GitLab API clients, Kafka, or Sarama.
- Usecases do not import repository/postgres, HTTP transport, Web transport, CLI code, S3 SDKs, Buf CLI adapters, GitLab API clients, Kafka/Sarama, or topic routing.
- State-changing usecases write outbox records transactionally and never publish to Kafka/Sarama directly.

Automated import-boundary tests enforce the mechanical parts of this contract:

- OSS `internal` and `cmd` production packages must not import paths such as `internal/enterprise`, `/enterprise/`, `enterprise/auth`, `enterprise/rbac`, `enterprise/license`, or `protoradar-enterprise`.
- `internal/identity` and `internal/authorization` must not import HTTP transport, CLI code, repository/postgres, OIDC/LDAP/RBAC concrete implementations, or enterprise packages.
- `internal/outbox` must stay transport-neutral; future Kafka/Sarama, webhook, SIEM, or enterprise dispatchers should implement `outbox.Dispatcher` outside the core package and be wired at a composition root.
- Config parsing remains centralized in `internal/config`; bootstrap consumes typed config and wires concrete implementations.

If a private build adds an OIDC provider, LDAP provider, advanced RBAC module, license checker, enterprise dispatcher, or alternate approval workflow, wire it from the downstream/private composition root. Do not add imports from OSS core packages back into private implementation packages.

The tests are not a replacement for review. They do not prove that transport code contains no business logic or that bootstrap contains only wiring; those constraints remain review responsibilities.

Authorizer denial tests are also guardrails rather than policy proof. They prove route wiring into `Authorizer`, not the correctness of future private RBAC rules or the strictness of `CommunityAuthorizer`.

## Explicit Non-Goals For Phase 11

Phase 11 does not add:

- OIDC implementation;
- LDAP implementation;
- SAML/session/browser login implementation;
- advanced RBAC;
- GitLab group sync;
- license management;
- enterprise-only code in the OSS repository;
- enterprise audit backends;
- custom approval policy DSL;
- advanced approval workflows.
