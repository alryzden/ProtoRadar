# Dependency Graph MVP

## Overview

ProtoRadar can detect direct protobuf module dependencies during publish and show which modules may be affected by a change. The dependency graph helps answer two practical questions:

- which upstream modules this module depends on;
- which downstream modules currently depend on this module and may need coordination before a breaking change is merged.

Community v1.0 stores dependency records as part of the publish flow and exposes them through the CLI, REST API, Basic Web UI, and GitLab merge request comments.

## How Dependencies Are Detected

ProtoRadar derives dependencies from descriptor metadata extracted from the published Buf workspace. The resolver compares the newly published module version with latest known provider modules.

Detected dependency sources:

- import-based dependencies: a file imports a `.proto` path that belongs to another published module, for example `import "user/v1/user.proto";`.
- field type references: a message field uses a type from another module, for example `user.v1.User account_owner = 2;`.
- method input/output type references: an RPC request or response type references a message from another module.

Dependency graph resolution lives in the application/usecase layer. Repositories persist and query the resulting graph records; renderers and transports only display query results.

## Ignored Dependencies

ProtoRadar deliberately ignores dependencies that do not represent a downstream relationship between distinct ProtoRadar modules:

- self dependencies: imports or type references that resolve to the same module being published;
- `google/protobuf` well-known imports;
- unknown external imports that are not currently modeled as ProtoRadar modules.

Unknown imports can still appear as unresolved dependencies when they look like application-owned proto files but no provider module is known.

## Unresolved Dependencies

Unresolved dependencies are recorded when ProtoRadar cannot map an import or symbol reference to exactly one provider module.

Common reasons:

- `provider_not_found`: no published provider module owns the imported file path or referenced symbol;
- `ambiguous_provider`: more than one provider module appears to own the same file path or symbol;
- external proto dependencies: third-party or remote imports that are not fully modeled in ProtoRadar yet.

Unresolved records are useful during onboarding because they show which imports need provider modules, module naming cleanup, or external dependency modeling later.

## Direct vs Transitive Dependencies

Community v1.0 MVP supports direct downstream dependencies only. If `billing-api` depends on `user-api`, `billing-api` appears as affected when querying `user-api`.

ProtoRadar does not yet traverse transitive dependency chains. If `frontend-api` depends on `billing-api` and `billing-api` depends on `user-api`, querying affected modules for `user-api` returns `billing-api`, not `frontend-api`. Transitive traversal is planned for a later phase.

## CLI Usage

Show the full direct dependency graph for a module:

```sh
protoradar module dependencies user-api
```

Show direct downstream modules that may be affected by changes to a module:

```sh
protoradar module affected user-api
```

The affected output reports downstream modules, their latest known version, dependency sources, and reasons. If no downstream consumers are known, ProtoRadar prints:

```text
No downstream modules are currently known to depend on this module.
```

## API Usage

All endpoints require `Authorization: Bearer <token>`.

Dependency graph for one module:

```text
GET /api/v1/modules/{module}/dependencies
```

Direct downstream affected modules:

```text
GET /api/v1/modules/{module}/affected
```

Affected modules for a stored breaking report:

```text
GET /api/v1/breaking-reports/{report_id}/affected-modules
```

The dependency graph response contains `upstream`, `downstream`, and `unresolved` sections. The affected-module responses contain the module name, latest version, dependency sources, and reasons.

## Web UI

The Basic Web UI includes a dependency page per module:

```text
/ui/modules/{module}/dependencies
```

The module details page links to this dependency page. The page shows:

- downstream consumers;
- upstream dependencies;
- unresolved dependencies.

Breaking report detail pages also include a Potentially Affected Modules section based on the same dependency read model.

## GitLab MR Comments

GitLab merge request comments now include a Potentially Affected Modules section. The MR runner fetches affected modules after the breaking check completes and passes the data to the Markdown renderer.

When affected modules exist, the comment renders a capped table with module, latest version, dependency sources, and reason. If no downstream modules are known, the comment says:

```text
No downstream modules are currently known to depend on this module.
```

The renderer only displays affected-module data. It does not compute dependency graph relationships.

## Example Scenario

Example modules live under `examples/repos`:

- `examples/repos/user-api`: defines reusable `user.v1` types;
- `examples/repos/billing-api`: imports `user/v1/user.proto` and references `user.v1.User`;
- `examples/repos/notification-api`: imports `user/v1/user.proto` and references `user.v1.User`.

Create the modules:

```sh
protoradar module create user-api \
  --description "User service protobuf contracts"

protoradar module create billing-api \
  --description "Billing service protobuf contracts"

protoradar module create notification-api \
  --description "Notification service protobuf contracts"
```

Publish the provider first, then publish consumers:

```sh
protoradar push user-api \
  --version v1.0.0 \
  --path examples/repos/user-api

protoradar push billing-api \
  --version v1.0.0 \
  --path examples/repos/billing-api

protoradar push notification-api \
  --version v1.0.0 \
  --path examples/repos/notification-api
```

Inspect dependencies for `user-api`:

```sh
protoradar module dependencies user-api
```

Inspect affected downstream modules:

```sh
protoradar module affected user-api
```

Open the Web UI dependency page:

```text
http://localhost:8080/ui/modules/user-api/dependencies
```

Note: the consumer example `buf.yaml` files use a placeholder Buf dependency name, `buf.build/protoradar/user-api`, to show the external provider relationship. In a real installation, use the Buf module reference that matches where your provider protos are published, or resolve imports through your organization's standard Buf dependency workflow.

For local validation of the example proto files without a remote Buf dependency, the graph directory also includes a multi-module Buf config:

```sh
buf lint examples/repos
buf build examples/repos
```

## Limitations

Community v1.0 intentionally keeps the graph MVP narrow:

- direct dependencies only;
- no transitive affected-module traversal yet;
- runtime inventory is modeled separately in Community v1.0; dependency graph edges are still schema-level dependencies;
- no generated client or SDK usage detection;
- ambiguous providers are recorded as unresolved instead of guessed;
- external dependencies are not fully modeled yet.

## Architecture Notes

The publish flow rebuilds dependency records for the published module version and writes a `ModuleDependenciesUpdated` outbox record transactionally with the publish state. Usecases do not publish directly to Kafka or Sarama. Kafka routing and publishing remain outside the usecase/domain layers.
