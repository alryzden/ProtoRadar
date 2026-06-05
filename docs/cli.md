# CLI Reference

The `protoradar` CLI talks to the ProtoRadar server over REST. It does not access PostgreSQL or object storage directly.

## Authentication

Login writes local credentials to `~/.config/protoradar/config.yaml` with `0600` permissions:

```sh
protoradar login --server http://localhost:8080 --token <token>
```

CI usage can avoid local config:

```sh
export PROTORADAR_SERVER_URL=https://protoradar.example.com
export PROTORADAR_TOKEN=<token>
```

Do not print or commit token values.

## Exit Codes

- `0`: success, safe result, or no breaking changes.
- `1`: breaking changes found by `check-breaking` or `gitlab mr-check`.
- `2`: invalid input, auth, network, server, config, tool, or internal error.

Runtime drift statuses are successful results and exit `0`.

## Commands

### `protoradar version`

Print build metadata:

```sh
protoradar version
```

Output includes version, commit, and build date.

### `protoradar module create`

```sh
protoradar module create user-api \
  --description "User service protobuf contracts" \
  --repository-url "https://gitlab.example.com/platform/user-api"
```

### `protoradar module list` / `protoradar list`

```sh
protoradar module list
protoradar list
```

Both list modules. `protoradar list` is the short form.

### `protoradar module link-gitlab`

```sh
protoradar module link-gitlab user-api \
  --project-id 12345 \
  --project-path platform/user-api \
  --gitlab-base-url https://gitlab.example.com
```

If flags are omitted in GitLab CI, the CLI can use `CI_PROJECT_ID`, `CI_PROJECT_PATH`, and `CI_SERVER_URL`.

### `protoradar push`

Publish a Buf-compatible workspace:

```sh
protoradar push user-api \
  --version v1.0.0 \
  --path examples/repos/user-api
```

The path must contain `buf.yaml` and at least one `.proto` file. The CLI uploads a source archive. The server runs Buf build/lint, stores source and Buf image artifacts, extracts descriptor metadata, and updates dependency graph data.

### `protoradar pull`

Download and extract the source archive for a published version:

```sh
protoradar pull user-api \
  --version v1.0.0 \
  --output ./tmp/user-api
```

Use `--force` to overwrite a non-empty output directory when supported by the command help.

### `protoradar check-breaking`

```sh
protoradar check-breaking user-api \
  --path examples/repos/user-api \
  --against latest \
  --target-ref feature/user-api \
  --report-file protoradar-breaking-report.txt
```

Exit `1` means breaking changes were found. It is not a server/tool failure.

### `protoradar gitlab mr-check`

Run a breaking check and update a GitLab merge request comment/status:

```sh
protoradar gitlab mr-check \
  --module user-api \
  --path . \
  --against latest \
  --gitlab-token "$PROTORADAR_GITLAB_TOKEN" \
  --report-file protoradar-mr-report.md
```

In GitLab CI, the command uses predefined CI variables for project, merge request, commit, and server URL. See [GitLab MR Bot](gitlab-mr-bot.md).

### `protoradar module dependencies`

Show upstream dependencies, downstream consumers, and unresolved dependencies:

```sh
protoradar module dependencies user-api
```

### `protoradar module affected`

Show direct downstream modules affected by a provider module:

```sh
protoradar module affected user-api
```

### `protoradar runtime report`

Report deployed module versions with flags:

```sh
protoradar runtime report \
  --service billing-service \
  --environment production \
  --git-commit abc1234 \
  --build-version demo-v1 \
  --module user-api@v1.0.0 \
  --module billing-api@v1.0.0
```

Or report from a YAML/JSON file:

```sh
protoradar runtime report --from-file examples/repos/billing-api/protoradar-runtime.yaml
```

With GitLab CI, defaults can come from `CI_PROJECT_NAME`, `CI_ENVIRONMENT_NAME`, `CI_COMMIT_SHA`, `CI_COMMIT_TAG`, and `CI_COMMIT_SHORT_SHA`.
