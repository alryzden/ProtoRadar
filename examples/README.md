# ProtoRadar Examples

## Single Module

- `examples/user-api`: minimal Buf workspace for publishing one module and running breaking checks.

## Dependency Graph Scenario

The Phase 7 dependency graph example uses three small modules:

- `examples/graph/user-api`: provider module defining `user.v1.User`, `GetUserRequest`, and `UserService`.
- `examples/graph/billing-api`: consumer module importing `user/v1/user.proto` and referencing `user.v1.User`.
- `examples/graph/notification-api`: consumer module importing `user/v1/user.proto` and referencing `user.v1.User`.

Publish the provider first, then consumers:

```sh
protoradar module create user-api --description "User service protobuf contracts"
protoradar module create billing-api --description "Billing service protobuf contracts"
protoradar module create notification-api --description "Notification service protobuf contracts"

protoradar push user-api --version v1.0.0 --path examples/graph/user-api
protoradar push billing-api --version v1.0.0 --path examples/graph/billing-api
protoradar push notification-api --version v1.0.0 --path examples/graph/notification-api
```

Inspect graph output:

```sh
protoradar module dependencies user-api
protoradar module affected user-api
```

Open the Basic Web UI dependency page:

```text
http://localhost:8080/ui/modules/user-api/dependencies
```

The consumer example `buf.yaml` files use `buf.build/protoradar/user-api` as a placeholder Buf dependency name. Replace it with your organization's real Buf module reference if you build these workspaces with Buf outside ProtoRadar demos.

The graph directory also includes `examples/graph/buf.yaml`, a local multi-module Buf config that can lint/build the three example proto trees together:

```sh
buf lint examples/graph
buf build examples/graph
```

## GitLab CI

- `examples/gitlab`: reusable GitLab CI templates for breaking checks, MR bot comments/statuses, and tag publishing.
