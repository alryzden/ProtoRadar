# GitLab CI Integration

## Overview

ProtoRadar can run from GitLab CI for protobuf governance workflows:

- simple breaking-change validation in merge request pipelines;
- full merge-request bot comments and commit statuses;
- publishing protobuf module versions from Git tags or release pipelines;
- reporting runtime inventory after deploys;
- linking ProtoRadar modules to GitLab projects.

All CI workflows use the `protoradar` CLI and the ProtoRadar REST API. CI jobs authenticate to ProtoRadar with the same bearer-token flow used by local CLI commands.

The full MR bot template also calls the GitLab API from the CLI. GitLab tokens are not stored in the ProtoRadar server.

## Template Choice

Use `examples/gitlab/protoradar-breaking-check.yml` when you only need a pass/fail CI job and a report artifact. This template does not call the GitLab API and does not require `PROTORADAR_GITLAB_TOKEN`.

Use `examples/gitlab/protoradar-mr-check.yml` when you want ProtoRadar to comment directly on the merge request and set a commit status. This template requires `PROTORADAR_GITLAB_TOKEN` or `GITLAB_TOKEN`.

Use `examples/gitlab/protoradar-publish.yml` to publish module versions from tag pipelines.

Use `examples/gitlab/protoradar-runtime-report.yml` to report deployed module versions after deployment jobs.

## Required CI Variables

Configure these variables in the GitLab project or group CI/CD settings:

- `PROTORADAR_SERVER_URL`: base URL of the ProtoRadar server, for example `https://protoradar.example.com`.
- `PROTORADAR_TOKEN`: ProtoRadar API token. Store it as a GitLab CI/CD variable; do not commit it.
- `PROTORADAR_MODULE`: ProtoRadar module name, for example `user-api`.
- `PROTORADAR_CLI_IMAGE`: container image containing the `protoradar` CLI on `PATH`.

For the MR bot template, also configure:

- `PROTORADAR_GITLAB_TOKEN`: GitLab API token allowed to comment on merge requests and set commit statuses. The CLI also accepts `GITLAB_TOKEN` when `PROTORADAR_GITLAB_TOKEN` is not set.

## Optional CI Variables

- `PROTORADAR_PROTO_PATH`: path to the Buf workspace. Defaults to `.`.
- `PROTORADAR_AGAINST`: breaking-check baseline. Defaults to `latest`.
- `PROTORADAR_REPORT_FILE`: report artifact path. Defaults to `protoradar-breaking-report.txt` for the simple check and `protoradar-mr-report.md` for the MR bot.
- `PROTORADAR_CLI_HTTP_TIMEOUT`: timeout for CLI HTTP calls to ProtoRadar and GitLab. Defaults to `30s` and must be a positive duration.
- `PROTORADAR_PUBLISH_VERSION`: publish version override. Defaults to `CI_COMMIT_TAG` in tag pipelines.
- `PROTORADAR_SERVICE`: runtime service name. Defaults to `CI_PROJECT_NAME` in runtime reporting.
- `PROTORADAR_ENVIRONMENT`: runtime environment. Defaults to `CI_ENVIRONMENT_NAME` in runtime reporting.
- `PROTORADAR_BUILD_VERSION`: runtime build version. Defaults to `CI_COMMIT_TAG` or `CI_COMMIT_SHORT_SHA`.
- `PROTORADAR_RUNTIME_FILE`: runtime report file path. Defaults to `protoradar-runtime.yaml`.
- `PROTORADAR_RUNTIME_MODULES`: space-separated `module@version` values for flag-based runtime reporting.

## CLI Image

All templates use:

```yaml
image: "$PROTORADAR_CLI_IMAGE"
```

The image contains the `protoradar` CLI and talks to the ProtoRadar server through the REST API. It does not run the server, PostgreSQL, MinIO, or any GitLab webhook service.

GitLab resolves the job image before `before_script`, so `PROTORADAR_CLI_IMAGE` is required. The templates do not provide a fallback registry image because this repository defines a local CLI image target but does not define a registry publishing workflow for an official image.

Build a local CLI image:

```sh
make docker-build-cli CLI_IMAGE=protoradar-cli:local
```

Or build directly:

```sh
docker build --target cli -t protoradar-cli:local .
```

Smoke test locally:

```sh
make docker-smoke-cli CLI_IMAGE=protoradar-cli:local
docker run --rm protoradar-cli:local version
```

Then set the CI variable to an image your GitLab runner can pull or already has locally:

```yaml
variables:
  PROTORADAR_CLI_IMAGE: "protoradar-cli:local"
```

For shared runners, build and push this image to your registry, then set `PROTORADAR_CLI_IMAGE`. Example values:

- `protoradar-cli:local` for a local runner that already has the image.
- `registry.example.com/platform/protoradar-cli:v1.0.0` for a private registry release tag.
- `registry.example.com/platform/protoradar-cli:abc1234` for an internally published commit image.

A private registry path is only an example, not a ProtoRadar requirement. Do not use a public `ghcr.io/...` image unless your organization has actually published one.

## Full MR Bot in Merge Requests

Use `examples/gitlab/protoradar-mr-check.yml` to comment on merge requests and set commit status.

Minimal `.gitlab-ci.yml`:

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

The template runs in merge request pipelines and executes `protoradar gitlab mr-check`. It passes GitLab predefined variables including `CI_SERVER_URL`, `CI_PROJECT_ID`, `CI_MERGE_REQUEST_IID`, `CI_COMMIT_SHA`, and `CI_JOB_URL`.

Exit behavior:

- no breaking changes: job passes with exit code `0`;
- breaking changes: job fails with exit code `1` and updates the MR comment/status;
- tool, auth, network, ProtoRadar API, GitLab API, config, or input errors: job fails with exit code `2`.

The Markdown report is uploaded as a GitLab artifact with `when: always` and `expire_in: 7 days`.

See [GitLab Merge Request Bot](gitlab-mr-bot.md) for token permissions, comment update behavior, commit status behavior, and troubleshooting.

## Simple Breaking Checks in Merge Requests

Use `examples/gitlab/protoradar-breaking-check.yml` to validate merge requests without GitLab API access.

Minimal `.gitlab-ci.yml`:

```yaml
include:
  - project: your-group/protoradar
    ref: main
    file: examples/gitlab/protoradar-breaking-check.yml

stages:
  - validate

variables:
  PROTORADAR_SERVER_URL: "https://protoradar.example.com"
  PROTORADAR_MODULE: "user-api"
  PROTORADAR_PROTO_PATH: "."
  PROTORADAR_AGAINST: "latest"
  PROTORADAR_CLI_IMAGE: "registry.example.com/platform/protoradar-cli:v1.0.0"

protoradar:breaking-check:
  extends: .protoradar-breaking-check
  stage: validate
```

The template runs:

```sh
protoradar check-breaking "$PROTORADAR_MODULE" \
  --path "${PROTORADAR_PROTO_PATH:-.}" \
  --against "${PROTORADAR_AGAINST:-latest}" \
  --target-ref "${CI_COMMIT_SHA:-local}" \
  --report-file "${PROTORADAR_REPORT_FILE:-protoradar-breaking-report.txt}"
```

The report file is uploaded as a GitLab artifact with `when: always`, so it is available for both passing checks and failed breaking-change checks.

## Publishing on Tags

Use `examples/gitlab/protoradar-publish.yml` to publish protobuf module versions from Git tags.

Minimal `.gitlab-ci.yml`:

```yaml
include:
  - project: your-group/protoradar
    ref: main
    file: examples/gitlab/protoradar-publish.yml

stages:
  - publish

variables:
  PROTORADAR_SERVER_URL: "https://protoradar.example.com"
  PROTORADAR_MODULE: "user-api"
  PROTORADAR_PROTO_PATH: "."
  PROTORADAR_CLI_IMAGE: "registry.example.com/platform/protoradar-cli:v1.0.0"

protoradar:publish:
  extends: .protoradar-publish
  stage: publish
```

The template runs only in tag pipelines. It publishes:

```sh
protoradar push "$PROTORADAR_MODULE" \
  --version "${PROTORADAR_PUBLISH_VERSION:-$CI_COMMIT_TAG}" \
  --path "${PROTORADAR_PROTO_PATH:-.}"
```

`PROTORADAR_PUBLISH_VERSION` can override the version. If it is not set, `CI_COMMIT_TAG` is used. The job fails clearly when neither value is available.

Publish at least one baseline version before expecting merge-request breaking checks to compare against `latest` or an explicit version. Breaking checks use the baseline version's stored Buf image artifact.

## Runtime Inventory After Deploy

Use `examples/gitlab/protoradar-runtime-report.yml` after deployment jobs to report which protobuf module versions are running.

File-based example:

```yaml
include:
  - project: your-group/protoradar
    ref: main
    file: examples/gitlab/protoradar-runtime-report.yml

stages:
  - deploy

variables:
  PROTORADAR_SERVER_URL: "https://protoradar.example.com"
  PROTORADAR_MODULE: "user-api"
  PROTORADAR_CLI_IMAGE: "registry.example.com/platform/protoradar-cli:v1.0.0"

deploy:production:
  stage: deploy
  environment:
    name: production
  script:
    - ./deploy.sh

protoradar:runtime-report:
  extends: .protoradar-runtime-report
  stage: deploy
  needs:
    - deploy:production
  variables:
    PROTORADAR_RUNTIME_FILE: "protoradar-runtime.yaml"
```

Flag-based example:

```yaml
protoradar:runtime-report:
  extends: .protoradar-runtime-report
  stage: deploy
  variables:
    PROTORADAR_SERVER_URL: "https://protoradar.example.com"
    PROTORADAR_MODULE: "billing-api"
    PROTORADAR_CLI_IMAGE: "registry.example.com/platform/protoradar-cli:v1.0.0"
    PROTORADAR_SERVICE: "billing-service"
    PROTORADAR_ENVIRONMENT: "production"
    PROTORADAR_RUNTIME_MODULES: "user-api@v1.2.0 billing-api@v1.4.0"
```

The template uses `PROTORADAR_SERVER_URL` and `PROTORADAR_TOKEN` for CLI auth. It also uses GitLab predefined variables including `CI_PROJECT_NAME`, `CI_ENVIRONMENT_NAME`, `CI_COMMIT_SHA`, `CI_COMMIT_TAG`, and `CI_COMMIT_SHORT_SHA`.

Runtime drift statuses are normal successful results. A report with `behind_latest` or `unknown_version` still exits `0` when the report is accepted.

## Module-to-GitLab Project Mapping

Link a ProtoRadar module to a GitLab project from CI:

```sh
protoradar module link-gitlab user-api \
  --project-id "$CI_PROJECT_ID" \
  --project-path "$CI_PROJECT_PATH" \
  --gitlab-base-url "$CI_SERVER_URL"
```

The CLI also reads these GitLab predefined variables as defaults when the flags are omitted:

- `CI_PROJECT_ID`;
- `CI_PROJECT_PATH`;
- `CI_SERVER_URL`.

The mapping records which GitLab project owns a ProtoRadar module. The mapping command does not call the GitLab API. It sends the mapping to ProtoRadar through the REST API.

## Token Security

Store tokens in GitLab CI/CD variables:

- mark `PROTORADAR_TOKEN` and `PROTORADAR_GITLAB_TOKEN` or `GITLAB_TOKEN` as masked;
- use protected variables for tokens that can publish versions;
- be careful with fork merge request pipelines, because untrusted code can run in CI depending on project settings;
- do not commit tokens to the repository;
- do not echo tokens in scripts.
- do not enable `set -x` around commands that read token environment variables.

ProtoRadar does not implement token scopes or RBAC yet. If scoped tokens or stricter RBAC are added in a future downstream build, use separate least-privilege tokens for read/check and publish workflows.

## Self-Managed GitLab

`gitlab_base_url` can point to GitLab SaaS or a self-managed GitLab instance, for example:

- `https://gitlab.com`;
- `https://gitlab.company.local`.

Use `CI_SERVER_URL` in GitLab CI when possible so the mapping and MR bot follow the project that is running the pipeline.

## Troubleshooting

`401 unauthorized from ProtoRadar`:
The `PROTORADAR_TOKEN` value is missing, invalid, expired, or not available to the pipeline. Check masked/protected variable settings and fork pipeline rules.

`PROTORADAR_CLI_IMAGE is required`:
Set `PROTORADAR_CLI_IMAGE` in project or group CI/CD variables. GitLab resolves the image before the script starts, so a missing image variable can fail before the template's validation messages run.

`image pull failed`:
The runner cannot pull `PROTORADAR_CLI_IMAGE`. Verify the image tag, registry credentials, runner network access, and whether the image was pushed to the registry. For local runners, verify the image exists on that runner host.

`401 or 403 from GitLab`:
The `PROTORADAR_GITLAB_TOKEN` or `GITLAB_TOKEN` value is missing, invalid, expired, unavailable, or lacks API permissions. Check token scope, project membership, and protected variable settings.

`404 module not found`:
Create the module first with `protoradar module create <module>` or verify `PROTORADAR_MODULE`.

`404 baseline not found`:
The version named by `PROTORADAR_AGAINST` does not exist for the module. Publish a baseline or use `latest` after a successful publish.

`409 baseline has no Buf image`:
The baseline exists but does not have a stored `buf_image` artifact. Publish a new baseline with the current `protoradar push` workflow.

`no buf.yaml found`:
`PROTORADAR_PROTO_PATH` is wrong or the workspace is missing `buf.yaml` at its root.

`protoradar: command not found` or wrong CLI behavior:
The CI image does not contain the `protoradar` CLI or contains an older binary. Override `PROTORADAR_CLI_IMAGE` with an image that has the expected CLI on `PATH`.

`TLS or certificate error`:
The CLI cannot validate the ProtoRadar server certificate or GitLab certificate. Use a server URL with the correct hostname, install the required CA certificates in the CLI image, or terminate TLS at a trusted proxy.

`server URL wrong`:
`PROTORADAR_SERVER_URL` must point to the ProtoRadar server base URL, not GitLab and not the Web UI path. For example, use `https://protoradar.example.com`, not `https://gitlab.example.com` or `https://protoradar.example.com/ui`.

Publish job has no tag/version:
The publish template runs on tag pipelines. If running it elsewhere, set `PROTORADAR_PUBLISH_VERSION` explicitly.
