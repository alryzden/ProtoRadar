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

## Token Last-Used Metadata Is Stale

Symptoms:

- API authentication succeeds.
- `api_tokens.last_used_at` is not updated or lags behind real requests.
- Server logs contain `api_token_mark_used_failed`.

`last_used_at` is operational metadata and is updated best-effort after token authentication. ProtoRadar does not deny an otherwise valid request only because this metadata write failed.

Check:

- database connectivity and write errors around the timestamp of the log event;
- whether the logged `api_token_id` matches the expected token row;
- repository/database errors emitted near `api_token_mark_used_failed`.

Raw token values are not logged for this condition.

## GitLab CI CLI Image Failures

Symptoms:

- GitLab reports `PROTORADAR_CLI_IMAGE is required`.
- GitLab reports image pull failures before the job script starts.
- CI logs show `protoradar: command not found`.
- `protoradar version` in `before_script` fails.

Check:

- `PROTORADAR_CLI_IMAGE` is set in project or group CI/CD variables.
- The image tag exists in a registry the runner can pull, or the local runner host already has the image.
- The image was built from the Dockerfile `cli` target: `docker build --target cli -t protoradar-cli:local .`.
- The image contains `protoradar` on `PATH`; verify with `docker run --rm protoradar-cli:local version`.
- Registry credentials are available to the runner when using a private registry.

This repository does not publish a public CLI image by default. Build and push the image to your registry, then set `PROTORADAR_CLI_IMAGE`, for example `registry.example.com/platform/protoradar-cli:v1.0.0`.

## GitLab CI Server URL Or TLS Failures

Symptoms:

- CLI reports connection refused, DNS errors, or `no such host`.
- CLI reports TLS certificate validation errors.
- CI uses the GitLab URL instead of the ProtoRadar server URL.

Check:

- `PROTORADAR_SERVER_URL` points to the ProtoRadar server base URL, not GitLab and not `/ui`.
- The GitLab runner can reach the ProtoRadar server over the network.
- HTTPS certificates match the hostname in `PROTORADAR_SERVER_URL`.
- Private CA certificates are installed in the CLI image or TLS is terminated at a trusted proxy.
- `PROTORADAR_TOKEN` is configured as a masked variable and is available to the pipeline type.

## CLI HTTP Timeout Failures

Symptoms:

- CLI reports a request timeout.
- GitLab MR bot exits with code `2` during ProtoRadar or GitLab API calls.
- CI jobs used to hang on network stalls and now fail after the configured timeout.

Check:

- `PROTORADAR_CLI_HTTP_TIMEOUT` is a positive duration such as `30s`, `45s`, or `2m`.
- The default CLI HTTP timeout is `30s` when no override is set.
- Increase the timeout only when large artifact uploads or slow GitLab/ProtoRadar networks need it.
- Do not set `PROTORADAR_CLI_HTTP_TIMEOUT=0s`; invalid values fail fast with a config error.

## Database Connection Errors

Symptoms:

- server fails at startup;
- `/readyz` returns `503` with `database: error`.

Check:

- `PROTORADAR_DATABASE_URL` host, port, database, user, password, and `sslmode`.
- PostgreSQL container health: `docker compose ps`.
- Network reachability from the server container.
- Database migrations in server logs.

## Server Config Validation Errors

Symptoms:

- server exits during startup before listening;
- logs or stderr mention an unsupported config field, invalid duration, invalid integer, missing required field, or non-positive value.

Check:

- Environment variables use the current `PROTORADAR_<SECTION>_<FIELD>` convention, for example `PROTORADAR_SERVER_HTTP_ADDR` and `PROTORADAR_SERVER_MAX_REQUEST_BODY_BYTES`.
- Old pre-koanf names such as `PROTORADAR_HTTP_ADDR` and `PROTORADAR_MAX_REQUEST_BODY_BYTES` are not supported aliases.
- If `protoradar-server --config <path>` is used, the file exists and contains only supported YAML keys.
- Durations use Go duration strings such as `5s`, `30s`, `1m`, or `2m30s`.
- Size values use numeric bytes only, not `100MiB` or `110MB`.
- Environment variables override YAML values, so check CI/CD variables and container environment in addition to the file.

Do not print full environments while debugging because they can contain API tokens, S3 credentials, bootstrap tokens, or GitLab tokens.

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

- `PROTORADAR_GITLAB_TOKEN` or `GITLAB_TOKEN` is set and masked.
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

## Runtime `deprecated_version`

`deprecated_version` is a successful runtime report result. It means the reported module/version exists in ProtoRadar and that module version has `deprecated_at` set.

Check:

- `GET /api/v1/modules/{module}/versions/{version}` for `deprecated_at`, `deprecated_by`, and `deprecation_reason`;
- whether the service is intentionally still deploying that version;
- whether a newer version should be deployed instead;
- the module name and version in the runtime report file or CI command.

Deprecation takes precedence over `behind_latest`. If a service reports an older version that is also deprecated, ProtoRadar reports `deprecated_version`, not `behind_latest`.

## Deprecated Version Can Still Be Pulled

Deprecation is metadata. It does not delete artifacts and does not block artifact download by itself.

Check:

- artifact download permissions and authentication;
- whether the artifact exists in object storage;
- whether the module version was published successfully before it was deprecated.

If `protoradar pull` fails for a deprecated version, troubleshoot it like any other artifact download failure unless server logs show a different cause.

## Unknown Version Is Not Deprecated

An unknown reported version is not marked `deprecated_version` because ProtoRadar has no stored module-version row to read deprecation metadata from.

Check:

- publish the module version first if it should be known;
- correct spelling and version formatting in runtime reports;
- avoid relying on deprecation metadata for versions that were never published.

The drift precedence is `unknown_version`, then `deprecated_version`, then `behind_latest`, then `up_to_date`.

## Reverse Or Remove Deprecation

Community currently exposes a deprecate workflow, but it does not expose an un-deprecate API or CLI command.

Check:

- whether deploying a newer non-deprecated version is the correct remediation;
- whether the deprecation was applied to the intended module/version;
- release notes or local operational policy before changing database state manually.

Do not manually update database deprecation columns as normal product usage. Manual database repair should be limited to controlled maintenance when you have confirmed the desired state and understand audit/outbox implications.

## Dependency Graph Is Empty

Check:

- provider module was published before consumer modules;
- consumer proto imports or type references point to provider symbols;
- publish completed successfully and metadata was extracted;
- Community v1.0 shows direct dependencies only.

## `/metrics` Missing Expected Series

Some counters appear only after the relevant operation runs. For example, publish metrics appear after publish requests, and dependency gauges update after dependency graph reads.

Outbox metrics appear after the outbox publisher runs. If `PROTORADAR_OUTBOX_PUBLISHER_ENABLED=false`, the publisher does not claim or dispatch records and publisher counters may remain absent or zero.

## Outbox Records Stuck Pending

Symptoms:

- `outbox_records.status = 'pending'`;
- `protoradar_outbox_pending_total` stays above zero;
- expected outbox logs are absent.

Check:

- `PROTORADAR_OUTBOX_PUBLISHER_ENABLED=true` if Community should drain records through the local logging dispatcher.
- `PROTORADAR_OUTBOX_PUBLISHER_POLL_INTERVAL` is not set unexpectedly high.
- `outbox_records.available_at <= now()` for records you expect to dispatch.
- server logs contain `outbox_publisher_started`.

Safe inspection query:

```sql
SELECT status, count(*)
FROM outbox_records
GROUP BY status
ORDER BY status;
```

Do not update rows manually unless you are repairing a known operational issue. Manual changes can break retry ordering or duplicate dispatch semantics.

## Outbox Records Stuck Processing

Symptoms:

- `outbox_records.status = 'processing'`;
- `claim_expires_at` is in the past;
- no publisher appears to be dispatching the record.

Check:

- the server process running the publisher is healthy;
- `claim_expires_at <= now()` for records that should be reclaimable;
- publisher logs for shutdowns or claim errors;
- database time is sane.

The PostgreSQL claim query can reclaim expired `processing` records. A non-expired `processing` record is intentionally skipped until its lease expires.

If server shutdown reports `outbox publisher shutdown timed out`, the publisher did not stop within the bounded close wait. With Community logging dispatch this should be unusual. For external/private dispatchers, verify that `Dispatch(ctx, record)` honors context cancellation promptly and that the dispatcher has finite network/client timeouts. Do not mark the in-flight record `published` manually unless you have independently confirmed delivery; wait for the claim lease to expire so it can be retried, or repair through a controlled maintenance process.

Safe inspection query:

```sql
SELECT id, event_type, attempts, claimed_at, claim_expires_at
FROM outbox_records
WHERE status = 'processing'
ORDER BY claim_expires_at
LIMIT 50;
```

## Outbox Records Failed Or Dead

Symptoms:

- `outbox_records.status = 'failed'` or `dead`;
- `protoradar_outbox_failed_total` or `protoradar_outbox_dead_total` is above zero;
- logs contain `outbox_dispatch_failed` or `outbox_record_marked_dead`.

Check:

- `last_error` for a bounded dispatcher error summary;
- `attempts` compared with `PROTORADAR_OUTBOX_PUBLISHER_MAX_ATTEMPTS`;
- `available_at` for the next retry time on failed records;
- dispatcher configuration. In Community this is the local `LoggingDispatcher`, not Kafka/Sarama or an external event bus.

Safe inspection query:

```sql
SELECT id, event_type, status, attempts, available_at, left(last_error, 300) AS last_error
FROM outbox_records
WHERE status IN ('failed', 'dead')
ORDER BY updated_at DESC
LIMIT 50;
```

Dead records are not retried automatically. If a record is dead, fix the root cause first. Then decide whether to replay, leave it as an audit trail, or repair state through a controlled migration/maintenance operation.

## Web UI Security Note

The Basic Web UI has no login/RBAC in Community v1.0. If it is reachable, users can inspect registry metadata. Put it behind a trusted network boundary or authentication proxy.

## Governance Duplicate Or Conflict Results

Approval request creation is idempotent per breaking report. Re-running `protoradar approvals request --report-id <id>` or the GitLab MR bot should return the existing approval request and exit successfully. It should not create duplicate requirements, audit events, or outbox records.

Approval decisions are final per requirement. If approve/reject returns HTTP `409 conflict` or the CLI exits `2`, check whether the requirement already has a decision. Repeated approve/reject, approve-after-reject, and reject-after-approve are expected conflicts, not internal server errors.

## Governance Invalid Values

Invalid module owner values are validation errors, not database/internal errors.

Check:

- `subject_type` or `--subject-type` is `user` or `team`;
- `role` or `--role` is `owner` or `maintainer`;
- REST responses use HTTP `400` with error code `validation_error`;
- CLI validation failures exit `2` before sending the request.

If an invalid value reaches PostgreSQL through a lower-level path, CHECK violations are mapped to stable application validation errors. User-facing API and CLI errors should not include SQLSTATE values or CHECK constraint names.

## Governance Actor Override Rejected

Symptoms:

- CLI governance command exits `2` after sending `--actor`.
- `protoradar gitlab mr-check` reports `server rejected governance actor override`.
- REST response indicates actor override is disabled or the requested actor differs from the authenticated principal.

Check:

- Remove `--actor`, `--governance-actor`, or request-body/query `actor` fields unless you are running a compatibility migration.
- Keep `PROTORADAR_GOVERNANCE_ACTOR_OVERRIDE_ENABLED=false` for normal deployments.
- If you intentionally need compatibility override in local/dev, set `PROTORADAR_GOVERNANCE_ACTOR_OVERRIDE_ENABLED=true` or `governance.actor_override_enabled: true` and understand that this weakens audit reliability.

Clients should omit actor by default. The server derives the governance audit actor from the authenticated principal, and raw API tokens are never used as actor strings.

## Approval Shows CI Token As Actor

This is expected when a GitLab CI job creates an approval request with `PROTORADAR_TOKEN`. Community authentication creates an API-token principal, and governance records that principal as the effective actor.

Check:

- The token name or token ID is the actor identity you expect for the CI job.
- The job is not passing deprecated actor override flags.
- MR comments display actors returned by the ProtoRadar API and do not rewrite them locally.

Community OSS does not map GitLab human users to approval actors.

## GitLab Username Is Not Shown As Actor

Community MR bot does not implement GitLab human identity mapping, GitLab group sync, OIDC, LDAP, or advanced RBAC. It cannot prove that a GitLab MR author, committer, approver, or username is the same identity as a ProtoRadar governance actor.

Check:

- Use the server-returned `decided_by` value for approval decisions.
- Do not expect MR comments to infer actors from GitLab predefined variables.
- Treat future GitLab human identity integration as enterprise/private work, not an OSS Community feature.
