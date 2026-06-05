# GitLab Merge Request Bot

## Overview

ProtoRadar can explain protobuf breaking-check results directly in GitLab merge requests. In a merge request pipeline, the `protoradar gitlab mr-check` command calls the ProtoRadar breaking-check API, renders a GitLab Markdown report, creates or updates a bot comment, and sets a GitLab commit status.

This integration is CLI-side. GitLab tokens are not stored in the ProtoRadar server, and ProtoRadar does not require GitLab OAuth/OIDC, a webhook server, group sync, or approvals.

## How It Works

Flow:

1. GitLab starts a merge request pipeline.
2. The CI job runs `protoradar gitlab mr-check`.
3. The CLI reads ProtoRadar credentials from `PROTORADAR_SERVER_URL` and `PROTORADAR_TOKEN`.
4. The CLI packages the Buf workspace and calls the ProtoRadar breaking-check REST API.
5. The CLI fetches direct downstream affected modules from ProtoRadar when the dependency graph API is available.
6. The CLI fetches runtime impact for the breaking report when a report ID is available.
7. The CLI calls the GitLab API with `PROTORADAR_GITLAB_TOKEN`.
8. The CLI creates or updates a GitLab merge request comment.
9. The CLI sets a GitLab commit status when status updates are enabled.
10. The CLI exits with stable code `0`, `1`, or `2`.

Breaking changes are normal governance results. They return exit code `1`, not a tool failure.

## Required CI Variables

Configure these variables in the GitLab project or group CI/CD settings:

- `PROTORADAR_SERVER_URL`: base URL of the ProtoRadar server, for example `https://protoradar.example.com`.
- `PROTORADAR_TOKEN`: ProtoRadar API token. Store it as a masked variable.
- `PROTORADAR_GITLAB_TOKEN`: GitLab token used by the bot to comment on merge requests and set commit statuses. Store it as a masked variable.
- `PROTORADAR_MODULE`: ProtoRadar module name, for example `user-api`.

Do not commit token values to the repository.

## GitLab Predefined Variables

The reusable template passes these GitLab predefined variables to the CLI:

- `CI_SERVER_URL`: GitLab base URL.
- `CI_PROJECT_ID`: numeric project ID.
- `CI_MERGE_REQUEST_IID`: merge request IID.
- `CI_COMMIT_SHA`: commit SHA being checked.
- `CI_JOB_URL`: CI job URL used as the commit-status target URL.

The MR bot template is intended for `merge_request_event` pipelines where these variables are available.

## Optional Variables

- `PROTORADAR_PROTO_PATH`: Buf workspace path. Defaults to `.`.
- `PROTORADAR_AGAINST`: breaking-check baseline. Defaults to `latest`.
- `PROTORADAR_CLI_IMAGE`: container image containing the `protoradar` CLI.
- `PROTORADAR_REPORT_FILE`: Markdown report artifact path. Defaults to `protoradar-mr-report.md`.

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
  PROTORADAR_CLI_IMAGE: "registry.example.com/platform/protoradar-cli:latest"
```

Configure `PROTORADAR_TOKEN` and `PROTORADAR_GITLAB_TOKEN` as masked GitLab CI/CD variables. Do not include token values in YAML.

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
- git commit.

If no runtime services are known, the comment says:

```text
No runtime services are currently known to use the affected module version.
```

Runtime impact lookup is best-effort. If the runtime impact API fails but the breaking check succeeds, the MR check keeps the breaking-check exit code and renders:

```text
Runtime impact could not be loaded. Check CI logs.
```

Runtime impact uses exact base-version matching in the Community v1.0 MVP. It does not perform SemVer range matching or transitive runtime impact analysis.

## Commit Status Behavior

Default status name/context:

```text
protoradar/breaking-check
```

Status mapping:

- passed check: `success`;
- breaking changes found: `failed`;
- tool, internal, ProtoRadar API, GitLab API, auth, network, or config error: `failed` if possible.

The CLI sets a running status before the breaking check when status updates are enabled. Use `--status=false` to disable all commit status updates. If status updates are disabled, commit-status failures cannot fail the job because no status calls are made.

The status target URL defaults to `CI_JOB_URL` in the CI template.

## Exit Codes

- `0`: safe change / no breaking changes.
- `1`: breaking changes found.
- `2`: internal, tool, auth, network, GitLab API, ProtoRadar API, config, or input error.

## Security Notes

- Do not commit `PROTORADAR_TOKEN` or `PROTORADAR_GITLAB_TOKEN`.
- Store both tokens in GitLab CI/CD variables.
- Mark token variables as masked.
- Use protected variables for publish-capable ProtoRadar tokens.
- Be careful with fork merge request pipelines. Depending on project settings, untrusted code may run in CI.
- Avoid exposing bot tokens to untrusted pipelines.
- Do not echo tokens in scripts.

ProtoRadar does not store GitLab bot tokens server-side in Community v1.0.

## Troubleshooting

`401 from GitLab`:
The GitLab token is missing, invalid, expired, or not available to the pipeline. Check `PROTORADAR_GITLAB_TOKEN`, masked/protected variable settings, and fork pipeline rules.

`403 from GitLab`:
The token exists but lacks permission to read the project, comment on merge requests, or set commit statuses. Check project membership, token role, and API scope.

`404 merge request not found`:
Verify `CI_PROJECT_ID`, `CI_MERGE_REQUEST_IID`, and `CI_SERVER_URL`. Ensure the job runs in a merge request pipeline, not a plain branch pipeline.

`No MR comment created`:
Check job logs for GitLab API errors, verify `PROTORADAR_GITLAB_TOKEN`, and confirm the template is `protoradar-mr-check.yml` rather than the simple `protoradar-breaking-check.yml`.

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
