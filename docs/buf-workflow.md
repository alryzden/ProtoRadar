# Buf-Compatible Workflow

ProtoRadar turns published protobuf module versions into Buf-compatible governance inputs. A publish request uploads a source archive, the server validates it with Buf, stores both source and descriptor artifacts, extracts descriptor metadata, and writes the publish event through the transactional outbox. Community v1.0 reuses the stored Buf image artifact as the baseline for breaking-change checks.

## Publish Requirements

`protoradar push` expects `--path` to point at a Buf workspace or module root. The root must contain:

- `buf.yaml`;
- `buf.lock` when the module has locked remote dependencies;
- one or more `.proto` files under the root or configured module paths.

The CLI packages `buf.yaml`, optional `buf.lock`, and `.proto` files recursively into a tar.gz source archive. It preserves relative paths and excludes common local/build directories such as `.git`, `node_modules`, `tmp`, `dist`, `build`, `generated`, and `vendor`.

The server treats `buf.yaml` as required by default. This is controlled by `buf.require_config` / `PROTORADAR_BUF_REQUIRE_CONFIG`.

## Server-Side Buf Build

Server-side Buf validation is authoritative. The server extracts the uploaded source archive into a temporary workspace using safe tar.gz extraction that rejects absolute paths, traversal paths, symlinks, hardlinks, non-regular entries, and oversized uncompressed content.

After extraction, the server runs:

```sh
buf build --as-file-descriptor-set -o -
```

The resulting binary descriptor set is stored as the `buf_image` artifact and parsed to extract descriptor metadata. Local CLI validation is intentionally not the source of truth.

## Buf Lock Behavior

If `buf.lock` is present in the source root, the CLI includes it and the server records that it was present along with its SHA-256 digest. If it is absent, publish can still proceed as long as `buf.yaml` and the Buf build are valid.

## Lint Modes

Configure lint behavior with `buf.lint_mode` / `PROTORADAR_BUF_LINT_MODE`:

- `disabled`: do not run `buf lint`;
- `warn`: run `buf lint`, allow publish on lint failure, and store warning status/report in the publish result/event;
- `enforce`: run `buf lint` and reject publish on lint failure.

Captured Buf output is limited by `buf.max_report_bytes` / `PROTORADAR_BUF_MAX_REPORT_BYTES`.

Breaking-check report capture and storage limits are configured separately:

- `breaking.max_report_bytes` / `PROTORADAR_BREAKING_MAX_REPORT_BYTES`;
- `breaking.max_changes` / `PROTORADAR_BREAKING_MAX_CHANGES`;
- `breaking.default_against` / `PROTORADAR_BREAKING_DEFAULT_AGAINST`.

## Stored Artifacts

Each module version stores multiple artifact records:

- `source_archive`: the uploaded tar.gz source archive;
- `buf_image`: the server-built binary descriptor image.

Storage keys follow this shape:

```text
modules/{module}/versions/{version}/source/sha256-{checksum}.tar.gz
modules/{module}/versions/{version}/buf-image/sha256-{checksum}.binpb
```

## Stored Descriptor Metadata

ProtoRadar persists descriptor metadata for future breaking-change checks and dependency analysis:

- files;
- package names;
- imports, including public/weak flags when available;
- services;
- methods;
- messages;
- fields;
- enums;
- enum values;
- summary counts.

Retrieve metadata with:

```sh
curl -sS \
  -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/modules/user-api/versions/v1.1.0/metadata
```

## Breaking Checks

Breaking checks compare a proposed workspace against the stored `buf_image` artifact from a previously published version. The proposed workspace is validated server-side, and `buf breaking` runs in the infrastructure Buf adapter.

See [Breaking Checks](breaking-checks.md) for CLI usage, REST endpoints, report structure, exit codes, and known limitations.

## Transactional Outbox

`ModuleVersionPublished` is written inside the same PostgreSQL transaction as the module version, artifact metadata, Buf config metadata, and descriptor metadata records. Usecases do not publish directly to Kafka, Sarama, or any broker.

The event payload includes source and Buf image artifact checksums/sizes, Buf config presence, Buf lock presence, lint status, and descriptor metadata summary counts. Raw API tokens are not included in events.

`BreakingReportCreated` is written inside the same PostgreSQL transaction as the breaking report and its changes. Breaking changes are normal check results, not internal failures.

`ModuleDependenciesUpdated` is written inside the same publish transaction as dependency graph records for the published module version.

## Known Limitations

- Dependency graph analysis is direct-only; transitive traversal is not implemented yet.
- Approval and waiver workflows are not implemented yet.
- Breaking diagnostic parsing is best-effort.
- Runtime usage and generated-client usage are not modeled yet.
- Generated SDK workflows are not implemented yet.
