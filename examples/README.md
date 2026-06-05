# ProtoRadar Examples

## Community v1.0 Repository Examples

The `examples/repos` directory contains three small repo-style Buf workspaces for local demos and smoke tests:

- `examples/repos/user-api`: provider module defining `user.v1.User`, request/response messages, and `UserService`.
- `examples/repos/billing-api`: consumer module importing `user/v1/user.proto` and referencing `user.v1.User` from billing messages and `BillingService`.
- `examples/repos/notification-api`: consumer module importing `user/v1/user.proto` and referencing `user.v1.User` from notification messages and `NotificationService`.

Publish order:

```sh
protoradar module create user-api --description "User service protobuf contracts"
protoradar module create billing-api --description "Billing service protobuf contracts"
protoradar module create notification-api --description "Notification service protobuf contracts"

protoradar push user-api --version v1.0.0 --path examples/repos/user-api
protoradar push billing-api --version v1.0.0 --path examples/repos/billing-api
protoradar push notification-api --version v1.0.0 --path examples/repos/notification-api
```

`billing-api` and `notification-api` use `buf.build/protoradar/user-api` as the example Buf dependency name. Replace it with your real Buf module reference if you validate these consumer workspaces against an external Buf registry.

For local syntax validation without an external Buf registry, build the parent workspace:

```sh
buf build examples/repos
```

Runtime report examples:

```sh
protoradar runtime report --from-file examples/repos/billing-api/protoradar-runtime.yaml
protoradar runtime report --from-file examples/repos/notification-api/protoradar-runtime.yaml
```

The runtime files report `billing-service` and `notification-service` running in `production` with their own module plus `user-api`.

Run the local end-to-end demo:

```sh
make demo
```

## Single Module

- `examples/repos/user-api`: minimal Buf workspace for publishing one module and running breaking checks.

## Dependency Graph Scenario

Community v1.0 repository examples use three small modules:

- `examples/repos/user-api`: provider module defining `user.v1.User`, `GetUserRequest`, and `UserService`.
- `examples/repos/billing-api`: consumer module importing `user/v1/user.proto` and referencing `user.v1.User`.
- `examples/repos/notification-api`: consumer module importing `user/v1/user.proto` and referencing `user.v1.User`.

Publish the provider first, then consumers:

```sh
protoradar module create user-api --description "User service protobuf contracts"
protoradar module create billing-api --description "Billing service protobuf contracts"
protoradar module create notification-api --description "Notification service protobuf contracts"

protoradar push user-api --version v1.0.0 --path examples/repos/user-api
protoradar push billing-api --version v1.0.0 --path examples/repos/billing-api
protoradar push notification-api --version v1.0.0 --path examples/repos/notification-api
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

The repos directory also includes `examples/repos/buf.yaml`, a local multi-module Buf config that can lint/build the three example proto trees together:

```sh
buf lint examples/repos
buf build examples/repos
```

## GitLab CI

- `examples/gitlab`: reusable GitLab CI templates for breaking checks, MR bot comments/statuses, tag publishing, and runtime inventory reporting.

## Runtime Inventory

- `examples/runtime/protoradar-runtime.yaml`: small runtime report file for `protoradar runtime report --from-file`.

Example:

```sh
protoradar runtime report --from-file examples/runtime/protoradar-runtime.yaml
```
