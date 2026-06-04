# Core Registry

The Core Registry stores versioned protobuf modules, publish metadata, and governance reports. Published versions are Buf-compatible: uploaded sources are validated server-side with Buf before the module version is persisted. Breaking checks compare proposed workspaces against stored Buf image artifacts from published baseline versions.

## What It Does

The registry provides:

- versioned protobuf module storage;
- PostgreSQL metadata persistence;
- S3/MinIO object storage;
- REST API endpoints;
- CLI commands for publishing and pulling artifacts;
- API token authentication;
- transactional outbox records for outgoing domain events;
- Buf config metadata persistence;
- descriptor metadata persistence;
- breaking report and breaking change persistence;
- module-to-GitLab project mapping;
- read-only Basic Web UI for inspecting modules, versions, metadata, artifacts, and reports.

Published versions now store two artifact kinds:

- `source_archive`: the uploaded tar.gz archive containing `buf.yaml`, optional `buf.lock`, and `.proto` files;
- `buf_image`: the server-built binary descriptor image produced by Buf.

## REST Endpoints

Authenticated with `Authorization: Bearer <token>`:

```text
POST /api/v1/modules
GET  /api/v1/modules
GET  /api/v1/modules/{module}
POST /api/v1/modules/{module}/versions
GET  /api/v1/modules/{module}/versions
GET  /api/v1/modules/{module}/versions/{version}
GET  /api/v1/modules/{module}/versions/{version}/metadata
GET  /api/v1/modules/{module}/versions/{version}/artifact
POST /api/v1/modules/{module}/breaking-checks
GET  /api/v1/modules/{module}/breaking-reports
PUT  /api/v1/modules/{module}/gitlab-project
GET  /api/v1/modules/{module}/gitlab-project
GET  /api/v1/breaking-reports/{report_id}
```

Bootstrap-token protected:

```text
POST /api/v1/tokens
```

Public:

```text
GET /healthz
GET /readyz
```

Read-only Web UI:

```text
GET /ui
GET /ui/modules
GET /ui/modules/{module}
GET /ui/modules/{module}/versions/{version}
GET /ui/breaking-reports
GET /ui/breaking-reports/{report_id}
```

## CLI Workflow

Create an API token with the bootstrap token:

```sh
curl -sS \
  -H "Authorization: Bearer local-bootstrap-token" \
  -H "Content-Type: application/json" \
  -d '{"name":"local-cli"}' \
  http://localhost:8080/api/v1/tokens
```

Log in:

```sh
protoradar login \
  --server http://localhost:8080 \
  --token <token>
```

Create a module:

```sh
protoradar module create user-api \
  --description "User service protobuf contracts" \
  --repository-url "https://gitlab.example.com/platform/user-api"
```

Publish a version from a Buf module root:

```sh
protoradar push user-api \
  --version v1.1.0 \
  --path examples/user-api
```

In GitLab CI, use `examples/gitlab/protoradar-publish.yml` to publish from tag pipelines. The publish version defaults to `CI_COMMIT_TAG`; set `PROTORADAR_PUBLISH_VERSION` to override it.

List modules:

```sh
protoradar list
```

View descriptor metadata:

```sh
curl -sS \
  -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/modules/user-api/versions/v1.1.0/metadata
```

For local/demo inspection, the same module, version, artifact, and descriptor metadata can be explored in the read-only Web UI at `http://localhost:8080/ui`. See [Basic Web UI](web-ui.md).

Pull a source archive:

```sh
protoradar pull user-api \
  --version v1.1.0 \
  --output ./tmp/user-api
```

If the output directory already exists and is not empty, use `--force`.

Check for breaking changes:

```sh
protoradar check-breaking user-api \
  --path examples/user-api \
  --against latest
```

Breaking checks require a published baseline version with a stored `buf_image` artifact. A `breaking` status is a completed check result, not an internal/server error.

Link a module to a GitLab project:

```sh
protoradar module link-gitlab user-api \
  --project-id "$CI_PROJECT_ID" \
  --project-path "$CI_PROJECT_PATH" \
  --gitlab-base-url "$CI_SERVER_URL"
```

This mapping records ownership between a ProtoRadar module and a GitLab project. It supports merge-request bot workflows, audit workflows, self-managed GitLab deployments, and future GitLab group sync.

## Artifact Safety

The CLI packages only `buf.yaml`, optional `buf.lock`, and `.proto` files. It preserves relative paths and excludes local/build directories such as `.git`, `node_modules`, `tmp`, `dist`, `build`, `generated`, and `vendor`.

On the server, source archive extraction rejects absolute paths, traversal paths such as `../evil.proto`, symlinks, hardlinks, non-regular entries, and archives that exceed the configured uncompressed size limit.

## Descriptor Metadata

The server extracts and persists descriptor metadata from the Buf image:

- files and package names;
- imports;
- services and methods;
- messages and fields;
- enums and enum values;
- summary counts.

This metadata supports future dependency analysis and complements breaking-change reports.

## Breaking Reports

Breaking reports record the result of comparing a proposed Buf workspace against a baseline version:

- `status`: `passed` or `breaking` for completed checks;
- `change_count`: number of stored changes;
- `changes`: structured diagnostics with file, symbol, rule, severity, and message where available;
- `human_summary`: reusable text report for CLI output.

The server runs `buf breaking` against the proposed workspace and the baseline version's stored `buf_image`. Diagnostic parsing is best-effort; raw uploaded source contents are not stored in the report or event.

Stored reports are also visible in the Basic Web UI under `/ui/breaking-reports`.

## GitLab Project Mapping

Each module can be linked to one GitLab project. A GitLab project, identified by `gitlab_base_url` and `gitlab_project_id`, can be linked to only one module.

The mapping stores:

- GitLab base URL;
- GitLab project ID;
- GitLab project path;
- timestamps for creation and update.

The base URL can be `https://gitlab.com` or a self-managed GitLab URL such as `https://gitlab.company.local`.

## Transactional Outbox

State-changing usecases write business data and outgoing event records in the same PostgreSQL transaction.

Events currently written:

- `ModuleCreated`
- `ModuleVersionPublished`
- `BreakingReportCreated`
- `ModuleGitLabProjectLinked`

`ModuleVersionPublished` is written after successful Buf build and inside the same transaction as version, artifact, Buf config, and descriptor metadata records. Usecases never publish directly to Kafka, Sarama, or another broker.

`BreakingReportCreated` is written inside the same transaction as `breaking_reports` and `breaking_changes`.

`ModuleGitLabProjectLinked` is written inside the same transaction as the `module_gitlab_projects` upsert.
