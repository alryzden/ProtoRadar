# Troubleshooting

## Auth Failures

Symptoms:

- HTTP `401 unauthorized`.
- CLI reports `unauthorized`.

Check:

- `PROTORADAR_SERVER_URL` points to the correct server.
- `PROTORADAR_TOKEN` is set in CI or `protoradar login` was run locally.
- The token was copied from `POST /api/v1/tokens` when it was created.
- The bootstrap token is used only for creating API tokens, not normal API calls.

Do not print token values in logs.

## Database Connection Errors

Symptoms:

- server fails at startup;
- `/readyz` returns `503` with `database: error`.

Check:

- `PROTORADAR_DATABASE_URL` host, port, database, user, password, and `sslmode`.
- PostgreSQL container health: `docker compose ps`.
- Network reachability from the server container.
- Database migrations in server logs.

## MinIO / S3 Errors

Symptoms:

- publish fails while storing artifacts;
- artifact download fails;
- startup config validation fails.

Check:

- `PROTORADAR_STORAGE_S3_ENDPOINT` is reachable from the server container.
- Bucket exists. Local Compose uses `minio-init` to create it.
- Access key and secret key are correct.
- `PROTORADAR_STORAGE_S3_USE_PATH_STYLE=true` for MinIO.
- Bucket permissions allow put/get.

## Buf Not Found Or Build Failed

Symptoms:

- publish returns `unprocessable_entity` or server error mentioning Buf;
- CLI output says Buf build/lint failed.

Check:

- `PROTORADAR_BUF_BINARY_PATH` points to `buf`.
- The server image includes Buf CLI.
- The publish path has `buf.yaml` at its root.
- The workspace builds locally: `buf build <path>`.
- Consumer example workspaces may require their Buf dependency configured; use `buf build examples/repos` for local validation of the bundled examples.

## Baseline Not Found

Symptoms:

- breaking check says module or baseline was not found.

Check:

- The module exists.
- The baseline version exists.
- `--against latest` only works after at least one successful publish.
- The token can read the module.

## No Buf Image For Baseline

Symptoms:

- breaking check reports baseline Buf image missing.

Check:

- The baseline version was published through ProtoRadar after Buf-compatible publishing was enabled.
- The `buf_image` artifact exists in metadata and object storage.
- Re-publish the baseline if this is a local demo environment.

## GitLab Token Errors

Symptoms:

- `protoradar gitlab mr-check` fails when commenting or setting status.

Check:

- `PROTORADAR_GITLAB_TOKEN` is set and masked.
- The token has API permissions for the project.
- Protected variable settings allow the pipeline to access the token.
- Fork pipeline restrictions are not blocking secrets.
- `CI_PROJECT_ID`, `CI_MERGE_REQUEST_IID`, and `CI_COMMIT_SHA` are present for MR jobs.

## Runtime `unknown_version`

`unknown_version` is a successful runtime report result. It means ProtoRadar could not match the reported module/version to a published version.

Check:

- module name spelling;
- version string spelling;
- publish order;
- whether the service is reporting the intended module list.

## Dependency Graph Is Empty

Check:

- provider module was published before consumer modules;
- consumer proto imports or type references point to provider symbols;
- publish completed successfully and metadata was extracted;
- Community v1.0 shows direct dependencies only.

## `/metrics` Missing Expected Series

Some counters appear only after the relevant operation runs. For example, publish metrics appear after publish requests, and dependency gauges update after dependency graph reads.

## Web UI Security Note

The Basic Web UI has no login/RBAC in Community v1.0. If it is reachable, users can inspect registry metadata. Put it behind a trusted network boundary or authentication proxy.
