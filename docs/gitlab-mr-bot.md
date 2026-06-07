# GitLab Merge Request Bot

## Overview

ProtoRadar can explain protobuf breaking-check results directly in GitLab merge requests. In a merge request pipeline, the `protoradar gitlab mr-check` command calls the ProtoRadar breaking-check API, renders a GitLab Markdown report, creates or updates a bot comment, and sets a GitLab commit status.

This integration is CLI-side. GitLab tokens are not stored in the ProtoRadar server, and ProtoRadar does not require GitLab OAuth/OIDC, a webhook server, or group sync. Community governance does not map GitLab human users to ProtoRadar approval actors.

## How It Works

Flow:

1. GitLab starts a merge request pipeline.
2. The CI job runs `protoradar gitlab mr-check`.
3. The CLI reads ProtoRadar credentials from `PROTORADAR_SERVER_URL` and `PROTORADAR_TOKEN`.
4. The CLI packages the Buf workspace and calls the ProtoRadar breaking-check REST API.
5. The CLI fetches direct downstream affected modules from ProtoRadar when the dependency graph API is available.
6. The CLI fetches runtime impact for the breaking report when a report ID is available.
7. The CLI fetches governance approval status for the breaking report when a report ID is available.
8. If `--governance=true` and no approval request exists, the CLI creates one through the ProtoRadar API.
9. The CLI calls the GitLab API with `PROTORADAR_GITLAB_TOKEN` or `GITLAB_TOKEN`.
10. The CLI creates or updates a GitLab merge request comment.
11. The CLI sets a GitLab commit status when status updates are enabled.
12. The CLI exits with stable code `0`, `1`, or `2`.

Breaking changes are normal governance results. They return exit code `1`, not a tool failure.

## Required CI Variables

Configure these variables in the GitLab project or group CI/CD settings:

- `PROTORADAR_SERVER_URL`: base URL of the ProtoRadar server, for example `https://protoradar.example.com`.
- `PROTORADAR_TOKEN`: ProtoRadar API token. Store it as a masked variable.
- `PROTORADAR_MODULE`: ProtoRadar module name, for example `user-api`.
- `PROTORADAR_CLI_IMAGE`: container image containing the `protoradar` CLI on `PATH`.
- `PROTORADAR_GITLAB_TOKEN`: GitLab token used by the bot to comment on merge requests and set commit statuses. Store it as a masked variable. The CLI also accepts `GITLAB_TOKEN` when `PROTORADAR_GITLAB_TOKEN` is not set.

Do not commit token values to the repository.

## GitLab Predefined Variables

The reusable template passes these GitLab predefined variables to the CLI:

- `CI_SERVER_URL`: GitLab base URL.
- `CI_PROJECT_ID`: numeric project ID.
- `CI_MERGE_REQUEST_IID`: merge request IID.
- `CI_COMMIT_SHA`: commit SHA being checked.
- `CI_JOB_URL`: CI job URL used as the commit-status target URL.

The MR bot template is intended for `merge_request_event` pipelines where these variables are available.

## Optional Settings

- `PROTORADAR_PROTO_PATH`: Buf workspace path. Defaults to `.`.
- `PROTORADAR_AGAINST`: breaking-check baseline. Defaults to `latest`.
- `PROTORADAR_REPORT_FILE`: Markdown report artifact path. Defaults to `protoradar-mr-report.md`.
- `PROTORADAR_CLI_HTTP_TIMEOUT`: timeout for CLI HTTP calls to ProtoRadar and GitLab. Defaults to `30s` and must be a positive duration.
- `PROTORADAR_GOVERNANCE`: when wired into your CI command as `--governance=true`, approved breaking changes can pass the MR check.
- `--governance-actor`: deprecated CLI compatibility flag. Omit it by default so the server records the authenticated ProtoRadar API-token principal. It only works when server actor override is enabled and should not be used to claim GitLab human identity.

## CLI Image

The MR bot template uses `image: "$PROTORADAR_CLI_IMAGE"`. The image contains the `protoradar` CLI and calls the ProtoRadar server REST API; it does not run the server or store GitLab tokens server-side.

GitLab resolves the job image before the job script starts, so set `PROTORADAR_CLI_IMAGE` in project or group CI/CD variables.

This repository has a local CLI image build target but no registry publishing workflow for an official image:

```sh
make docker-build-cli CLI_IMAGE=protoradar-cli:local
```

Direct Docker build and smoke test:

```sh
docker build --target cli -t protoradar-cli:local .
docker run --rm protoradar-cli:local version
```

Use that local image only with runners that can access it. For shared runners, build and push this image to your registry, then set `PROTORADAR_CLI_IMAGE`, for example `registry.example.com/platform/protoradar-cli:v1.0.0`. Do not use a public image name unless your organization has actually published it.

## GitLab Token Permissions

Use a project access token, group access token, or bot user token with GitLab API access. The token must be able to:

- read the target project and merge request;
- create and update merge request notes;
- set commit statuses for the checked commit.

Exact permissions can differ between GitLab.com and self-managed GitLab installations. If the bot receives `401` or `403` responses, verify token scope, project membership, and whether the token is available to the current pipeline type.

## Minimal `.gitlab-ci.yml`

```yaml
include:
  - project: platform/protoradar
    ref: v1.0.0
    file:
      - /examples/gitlab/protoradar-mr-check.yml

stages:
  - validate

variables:
  PROTORADAR_SERVER_URL: "https://protoradar.example.com"
  PROTORADAR_MODULE: "user-api"
  PROTORADAR_PROTO_PATH: "."
  PROTORADAR_AGAINST: "latest"
  PROTORADAR_CLI_IMAGE: "registry.example.com/platform/protoradar-cli:v1.0.0"
```

Configure `PROTORADAR_TOKEN` and `PROTORADAR_GITLAB_TOKEN` or `GITLAB_TOKEN` as masked GitLab CI/CD variables. Do not include token values in YAML.

The template runs:

```sh
protoradar gitlab mr-check \
  --module "$PROTORADAR_MODULE" \
  --path "${PROTORADAR_PROTO_PATH:-.}" \
  --against "${PROTORADAR_AGAINST:-latest}" \
  --gitlab-base-url "$CI_SERVER_URL" \
  --project-id "$CI_PROJECT_ID" \
  --merge-request-iid "$CI_MERGE_REQUEST_IID" \
  --commit-sha "$CI_COMMIT_SHA" \
  --target-ref "$CI_COMMIT_SHA" \
  --status-target-url "$CI_JOB_URL" \
  --governance="${PROTORADAR_GOVERNANCE:-false}" \
  --report-file "${PROTORADAR_REPORT_FILE:-protoradar-mr-report.md}"
```

## Comment Update Behavior

ProtoRadar includes a hidden marker at the top of each MR bot comment:

```html
<!-- protoradar:mr-check module={module_name} -->
```

When the job runs, ProtoRadar lists existing merge request notes and looks for this marker. If it finds one marker comment, it updates that comment. If it finds multiple marker comments, it updates the latest one when timestamps are available, otherwise the last matching note in the list.

Duplicate comments are avoided by updating the marker comment. Community v1.0 does not delete older duplicates. If someone manually removes the marker from the comment, the next run may create a new comment.

## Potentially Affected Modules

MR comments include a Potentially Affected Modules section. After the breaking check completes, the runner asks ProtoRadar for direct downstream modules that currently depend on the checked module.

When downstream consumers are known, the comment renders a capped table with:

- module;
- latest version;
- dependency sources;
- reason.

If none are known, the comment says:

```text
No downstream modules are currently known to depend on this module.
```

The affected-module lookup is best-effort so a dependency graph API problem does not hide the primary breaking-check result. The Markdown renderer only displays affected-module data; dependency graph computation stays in ProtoRadar application/query services.

## Runtime Impact

MR comments include a Runtime impact section after Potentially Affected Modules. After the breaking check completes, the runner asks ProtoRadar for services and environments currently using the exact base module version from the breaking report.

When runtime usage is known, the comment renders a capped table with:

- service;
- environment;
- used module version;
- build version;
- git commit;
- server-calculated runtime drift status and reason when returned by the runtime impact API.

If a matching runtime usage is deprecated, the table shows `deprecated_version` in the runtime drift column. The MR bot does not calculate drift locally, does not query module version metadata directly, and does not create enterprise runtime alerts.

Example row:

```markdown
| `billing-service` | `production` | `user-api@v1.0.0` | `2026.06.04-15` | `abc1234` | `deprecated_version` | deprecated_version |
```

If no runtime services are known, the comment says:

```text
No runtime services are currently known to use the affected module version.
```

Runtime impact lookup is best-effort. If the runtime impact API fails but the breaking check succeeds, the MR check keeps the breaking-check exit code and renders:

```text
Runtime impact could not be loaded. Check CI logs.
```

Runtime impact uses exact base-version matching in the Community v1.0 MVP. It does not perform SemVer range matching or transitive runtime impact analysis.

## Governance Section

MR comments include a Governance section when approval status is available or governance mode creates an approval request. Approval request creation is idempotent per breaking report, so repeated or concurrent MR bot runs reuse the existing approval request instead of creating duplicate requirements, audit events, or outbox records. The runner fetches governance state through the ProtoRadar REST API; the Markdown renderer only displays the returned state and does not compute policy.

Community governance actors for CI-created approval requests come from the authenticated `PROTORADAR_TOKEN` principal on the ProtoRadar server. They are not GitLab human users, and the MR bot does not infer actor identity from GitLab variables such as author, committer, approver, or username. If you explicitly pass `--governance-actor`, the server rejects it unless `governance.actor_override_enabled=true`; use that only for migration compatibility.

MR comments display governance status, decisions, and `decided_by` values returned by the ProtoRadar API. If the API does not return an actor value, the renderer reports that it was not returned instead of inventing one from GitLab context.

Pending example:

```markdown
### Governance

Status: ⏳ Approval required

| Requirement | Target | Status | Reason |
|---|---|---|---|
| Module owner approval | `user-api` | Pending | Breaking changes require approval from module owner. |
```

Other statuses render as:

- `Status: ✅ Approved`
- `Status: ❌ Rejected`
- `Status: ✅ Approval not required`

When a required module has no configured owner, the section includes a warning similar to:

```text
Approval required, but no owners are configured for `user-api`.
```

Markdown table cells are escaped and known token/secret patterns are redacted.

## Commit Status Behavior

Default status name/context:

```text
protoradar/breaking-check
```

Status mapping:

- passed check: `success`;
- breaking changes found with `--governance=false`: `failed`;
- breaking changes with `--governance=true` and approval status `approved`: `success`;
- breaking changes with `--governance=true` and approval status `pending` or `rejected`: `failed`;
- tool, internal, ProtoRadar API, GitLab API, auth, network, or config error: `failed` if possible.

The CLI sets a running status before the breaking check when status updates are enabled. Use `--status=false` to disable all commit status updates. If status updates are disabled, commit-status failures cannot fail the job because no status calls are made.

The status target URL defaults to `CI_JOB_URL` in the CI template.

## Exit Codes

- `0`: safe change, no breaking changes, approval not required, or approved breaking change in governance mode.
- `1`: breaking changes found without governance mode, or pending/rejected governance status in governance mode.
- `2`: internal, tool, auth, network, GitLab API, ProtoRadar API, config, or input error.

`--governance=false` preserves the old compatibility semantics. `--governance=true` changes only the breaking-change decision gate so approved breaking changes can pass.

## Security Notes

- Do not commit `PROTORADAR_TOKEN`, `PROTORADAR_GITLAB_TOKEN`, or `GITLAB_TOKEN`.
- Store both tokens in GitLab CI/CD variables.
- Mark token variables as masked.
- Use protected variables for publish-capable ProtoRadar tokens.
- Be careful with fork merge request pipelines. Depending on project settings, untrusted code may run in CI.
- Avoid exposing bot tokens to untrusted pipelines.
- Do not echo tokens in scripts.
- Do not enable `set -x` around commands that read token environment variables.

ProtoRadar does not store GitLab bot tokens server-side in Community v1.0.

Community governance actor identity for CI-created requests is the ProtoRadar API-token principal, not the GitLab human user. GitLab human identity integration and GitLab group sync are not implemented in OSS.

## Troubleshooting

`401 from GitLab`:
The GitLab token is missing, invalid, expired, or not available to the pipeline. Check `PROTORADAR_GITLAB_TOKEN` or `GITLAB_TOKEN`, masked/protected variable settings, and fork pipeline rules.

`PROTORADAR_CLI_IMAGE is required`:
Set `PROTORADAR_CLI_IMAGE` in project or group CI/CD variables. GitLab may fail image resolution before the template's `before_script` validation can print a clearer error.

`image pull failed`:
The runner cannot pull the CLI image. Verify the tag, registry credentials, runner network, and whether the image was pushed. Local image tags such as `protoradar-cli:local` work only on runners that already have that image.

`protoradar: command not found`:
The configured image does not contain the `protoradar` CLI on `PATH`. Rebuild the CLI image with `docker build --target cli` or set `PROTORADAR_CLI_IMAGE` to the correct pushed image.

`TLS or certificate error`:
The CLI cannot validate the ProtoRadar server or GitLab certificate. Use the correct hostname in `PROTORADAR_SERVER_URL`/`CI_SERVER_URL`, install the required CA certificates in the CLI image, or terminate TLS at a trusted proxy.

`403 from GitLab`:
The token exists but lacks permission to read the project, comment on merge requests, or set commit statuses. Check project membership, token role, and API scope.

`404 merge request not found`:
Verify `CI_PROJECT_ID`, `CI_MERGE_REQUEST_IID`, and `CI_SERVER_URL`. Ensure the job runs in a merge request pipeline, not a plain branch pipeline.

`No MR comment created`:
Check job logs for GitLab API errors, verify `PROTORADAR_GITLAB_TOKEN` or `GITLAB_TOKEN`, and confirm the template is `protoradar-mr-check.yml` rather than the simple `protoradar-breaking-check.yml`.

`Duplicate comments`:
ProtoRadar updates comments by hidden marker. If the marker was removed manually, the next run may create a new comment. Community v1.0 does not delete duplicate comments.

`Commit status not visible`:
Verify status updates are not disabled with `--status=false`, check token permissions, and confirm the status is attached to `CI_COMMIT_SHA`.

`CI variables are empty`:
Use `merge_request_event` pipelines. GitLab MR variables such as `CI_MERGE_REQUEST_IID` are not available in every pipeline type.

`no buf.yaml found`:
`PROTORADAR_PROTO_PATH` points to the wrong directory or the Buf workspace is missing `buf.yaml` at its root.

`baseline not found`:
The version named by `PROTORADAR_AGAINST` does not exist. Publish a baseline first or use `latest` after a successful publish.

`baseline has no Buf image`:
The baseline version does not have a stored `buf_image` artifact. Publish a new baseline with the current `protoradar push` workflow.

`runtime impact could not be loaded`:
The breaking check completed, but the runtime impact lookup failed. Check ProtoRadar API reachability, token permissions, and server logs. The breaking-check exit code is still based on the compatibility result.

`governance approval status failed`:
The breaking check completed, but governance mode could not fetch or create approval state. In `--governance=true` mode this exits `2`. Creating approval state is idempotent, so normal repeated MR bot runs should return the existing request rather than fail. Check ProtoRadar API reachability, token permissions, and server logs.

`server rejected governance actor override`:
The job passed `--governance-actor`, but the server does not allow actor override. Remove the flag so ProtoRadar records the authenticated `PROTORADAR_TOKEN` principal, or enable server actor override only for migration compatibility.

`approval shows CI token as actor`:
This is expected in Community governance. CI-created approval requests are authenticated with `PROTORADAR_TOKEN`, so the server records that API-token principal as the actor.

`GitLab username is not shown as actor`:
Community MR bot does not perform GitLab user identity mapping or group sync. MR comments show actors returned by the ProtoRadar API and do not derive approval actors from GitLab usernames.
