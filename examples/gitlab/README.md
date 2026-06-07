# ProtoRadar GitLab CI Examples

These examples show how to run ProtoRadar from GitLab CI without interactive login. The CLI reads `PROTORADAR_SERVER_URL` and `PROTORADAR_TOKEN` from environment variables, so CI jobs do not need to write credentials to disk.

## Files

- `protoradar-breaking-check.yml`: reusable hidden job `.protoradar-breaking-check` for simple merge request pass/fail validation without GitLab API access.
- `protoradar-mr-check.yml`: reusable job `protoradar:mr-check` for the full merge request bot. It posts or updates a GitLab MR comment and sets a commit status.
- `protoradar-publish.yml`: reusable hidden job `.protoradar-publish` for publishing module versions from tag pipelines.
- `protoradar-runtime-report.yml`: reusable hidden job `.protoradar-runtime-report` for reporting runtime inventory after deployment.
- `protoradar-full.yml`: combined local example that includes the MR bot template and publish template.
- `consumer/.gitlab-ci.yml`: example consuming project configuration for the MR bot.

## Breaking Check vs MR Bot

Use `protoradar-breaking-check.yml` when you only want the GitLab job to pass or fail and upload a plain text report artifact. It only needs ProtoRadar API credentials.

Use `protoradar-mr-check.yml` when you want ProtoRadar to explain the result directly in the merge request. It requires both ProtoRadar credentials and a GitLab API token. The bot job:

- calls `protoradar gitlab mr-check`;
- runs only in `merge_request_event` pipelines;
- creates or updates one marker comment for the module;
- avoids noisy duplicate comments by updating the existing marker comment;
- sets the commit status named `protoradar/breaking-check` by default;
- uploads the Markdown report artifact with `when: always`.

## Required CI/CD Variables

Set these in the GitLab project or group CI/CD variables:

- `PROTORADAR_SERVER_URL`: base URL of the ProtoRadar server.
- `PROTORADAR_TOKEN`: ProtoRadar API token. Store it as a masked variable; do not commit it.
- `PROTORADAR_MODULE`: ProtoRadar module name, for example `user-api`.
- `PROTORADAR_CLI_IMAGE`: container image containing the `protoradar` CLI on `PATH`.

For the MR bot template, also set:

- `PROTORADAR_GITLAB_TOKEN`: GitLab token allowed to create/update merge request notes and set commit statuses. Store it as a masked variable; do not commit it. The CLI also accepts `GITLAB_TOKEN` when `PROTORADAR_GITLAB_TOKEN` is not set.

For the runtime report template, provide either `PROTORADAR_RUNTIME_FILE` or `PROTORADAR_RUNTIME_MODULES`.

## Optional Variables

- `PROTORADAR_PROTO_PATH`: Buf workspace path. Defaults to `.`.
- `PROTORADAR_AGAINST`: breaking-check baseline. Defaults to `latest`.
- `PROTORADAR_REPORT_FILE`: report artifact path. Defaults to `protoradar-mr-report.md` for the MR bot and `protoradar-breaking-report.txt` for the simple check.
- `PROTORADAR_PUBLISH_VERSION`: publish version override. Defaults to `CI_COMMIT_TAG` in tag pipelines.
- `PROTORADAR_SERVICE`: runtime service name. Defaults to `CI_PROJECT_NAME` in runtime reporting.
- `PROTORADAR_ENVIRONMENT`: runtime environment. Defaults to `CI_ENVIRONMENT_NAME` in runtime reporting.
- `PROTORADAR_BUILD_VERSION`: runtime build version. Defaults to `CI_COMMIT_TAG` or `CI_COMMIT_SHORT_SHA`.
- `PROTORADAR_RUNTIME_FILE`: runtime report file path. Defaults to `protoradar-runtime.yaml`.
- `PROTORADAR_RUNTIME_MODULES`: space-separated `module@version` values for flag-based runtime reporting.
## CLI Image

The templates use `image: "$PROTORADAR_CLI_IMAGE"`. GitLab resolves the job image before `before_script`, so `PROTORADAR_CLI_IMAGE` is required.

This repository has a local CLI image target but no registry publishing workflow for an official image:

```sh
make docker-build-cli CLI_IMAGE=protoradar-cli:local
```

Use a local image only with runners that can access it. For shared runners, publish your own CLI image and set `PROTORADAR_CLI_IMAGE` to that image.

Example override:

```yaml
variables:
  PROTORADAR_CLI_IMAGE: "registry.example.com/platform/protoradar-cli:v1.0.0"
```

## Include the MR Bot Template

```yaml
include:
  - project: platform/protoradar
    ref: v0.5.0
    file:
      - /examples/gitlab/protoradar-mr-check.yml

stages:
  - validate

variables:
  PROTORADAR_SERVER_URL: "https://protoradar.example.com"
  PROTORADAR_MODULE: "user-api"
  PROTORADAR_PROTO_PATH: "."
  PROTORADAR_AGAINST: "latest"
```

Configure `PROTORADAR_TOKEN` and `PROTORADAR_GITLAB_TOKEN` or `GITLAB_TOKEN` as masked GitLab CI/CD variables. Do not put token values in `.gitlab-ci.yml`.

The MR bot template passes GitLab predefined variables to the CLI:

- `CI_SERVER_URL`;
- `CI_PROJECT_ID`;
- `CI_MERGE_REQUEST_IID`;
- `CI_COMMIT_SHA`;
- `CI_JOB_URL`.

The job runs:

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

Exit behavior:

- `0`: no breaking changes;
- `1`: breaking changes found;
- `2`: tool, auth, network, ProtoRadar API, GitLab API, config, or input error.

The Markdown report is uploaded as an artifact with `when: always` and `expire_in: 7 days`.

## Simple Breaking-Change Validation

`protoradar-breaking-check.yml` runs only in merge request pipelines and writes a report artifact with `when: always` and `expire_in: 7 days`. It does not need `PROTORADAR_GITLAB_TOKEN` and does not call the GitLab API.

Example:

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

## Publishing Tag Versions

`protoradar-publish.yml` runs only in tag pipelines. The publish version defaults to `CI_COMMIT_TAG`; set `PROTORADAR_PUBLISH_VERSION` to override it.

Example:

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

The job runs:

```sh
protoradar push "$PROTORADAR_MODULE" \
  --version "${PROTORADAR_PUBLISH_VERSION:-$CI_COMMIT_TAG}" \
  --path "${PROTORADAR_PROTO_PATH:-.}"
```

Publish a baseline version before relying on merge-request breaking checks that use `latest`.

## Runtime Inventory Reporting

`protoradar-runtime-report.yml` reports deployed protobuf module versions after a deployment. It uses `PROTORADAR_SERVER_URL` and `PROTORADAR_TOKEN` for CLI auth and does not require a GitLab API token.

File-based example:

```yaml
include:
  - project: your-group/protoradar
    ref: main
    file: examples/gitlab/protoradar-runtime-report.yml

stages:
  - deploy

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

The file can look like:

```yaml
service_name: billing-service
environment: production
git_commit: abc1234
build_version: 2026.06.04-15
modules:
  - module: user-api
    version: v1.2.0
  - module: billing-api
    version: v1.4.0
```

Flag-based example:

```yaml
protoradar:runtime-report:
  extends: .protoradar-runtime-report
  stage: deploy
  variables:
    PROTORADAR_SERVICE: "billing-service"
    PROTORADAR_ENVIRONMENT: "production"
    PROTORADAR_RUNTIME_MODULES: "user-api@v1.2.0 billing-api@v1.4.0"
```

The template uses GitLab predefined variables:

- `CI_PROJECT_NAME`;
- `CI_ENVIRONMENT_NAME`;
- `CI_COMMIT_SHA`;
- `CI_COMMIT_TAG`;
- `CI_COMMIT_SHORT_SHA`.

Runtime reports are snapshots. Drift results such as `behind_latest` and `unknown_version` do not fail the job when the report is accepted.

## Module-to-GitLab Project Mapping

Use the CLI to link a ProtoRadar module to the GitLab project that owns it:

```sh
protoradar module link-gitlab user-api \
  --project-id "$CI_PROJECT_ID" \
  --project-path "$CI_PROJECT_PATH" \
  --gitlab-base-url "$CI_SERVER_URL"
```

The CLI can also read `CI_PROJECT_ID`, `CI_PROJECT_PATH`, and `CI_SERVER_URL` as defaults when those flags are omitted.

Mapping is useful for audit, ownership discovery, self-managed GitLab support, and future downstream integrations.

## Token Storage

Store tokens in GitLab CI/CD variables:

- mark `PROTORADAR_TOKEN` and `PROTORADAR_GITLAB_TOKEN` or `GITLAB_TOKEN` as masked;
- use protected variables for tokens that can publish versions;
- be careful with fork merge request pipelines, because untrusted code can run depending on project settings;
- do not commit tokens;
- do not echo tokens in scripts.
