package cli

import (
	"errors"
	"fmt"
	"strings"
)

func (app App) usage() error {
	return errors.New("usage: protoradar <login|version|module|push|check-breaking|gitlab|pull|list|runtime|approvals>")
}

func (app App) printHelp(topic string) {
	fmt.Fprint(app.output(), commandHelp(strings.TrimSpace(topic)))
}

func hasHelp(args []string) bool {
	return len(args) > 0 && isHelp(args[0])
}

func isHelp(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func commandHelp(topic string) string {
	switch topic {
	case "login":
		return `Usage:
  protoradar login --server <url> --token <token>

Stores credentials for later CLI commands. The token is never printed.
`
	case "module":
		return `Usage:
  protoradar module <create|list|link-gitlab|dependencies|affected|owners|version> [flags]
`
	case "module create":
		return `Usage:
  protoradar module create <module> [--description <text>] [--repository-url <url>]

Example:
  protoradar module create user-api --description "User API contracts"
`
	case "module link-gitlab":
		return `Usage:
  protoradar module link-gitlab <module> --project-id <id> --project-path <path> --gitlab-base-url <url>
`
	case "module dependencies":
		return `Usage:
  protoradar module dependencies <module>

Shows direct upstream, downstream, and unresolved protobuf dependencies.
`
	case "module affected":
		return `Usage:
  protoradar module affected <module>

Shows direct downstream modules currently known to depend on the module.
`
	case "module owners":
		return `Usage:
  protoradar module owners <list|add|remove> [flags]
`
	case "module owners list":
		return `Usage:
  protoradar module owners list <module>
`
	case "module owners add":
		return `Usage:
  protoradar module owners add <module> --subject-type user|team --subject <name> --role owner|maintainer [--actor <actor>]

--actor is deprecated and only works when server actor override is enabled. Omit it to use the authenticated principal.
`
	case "module owners remove":
		return `Usage:
  protoradar module owners remove <module> --owner-id <id> [--actor <actor>]

--actor is deprecated and only works when server actor override is enabled. Omit it to use the authenticated principal.
`
	case "module version":
		return `Usage:
  protoradar module version <deprecate> [flags]
`
	case "module version deprecate":
		return `Usage:
  protoradar module version deprecate <module> <version> [--reason <text>]

The server records the authenticated principal as deprecated_by.
`
	case "list":
		return `Usage:
  protoradar list
  protoradar module list
`
	case "push":
		return `Usage:
  protoradar push <module> --version <version> --path <buf-workspace>
`
	case "pull":
		return `Usage:
  protoradar pull <module> --version <version> --output <directory> [--force]
`
	case "check-breaking":
		return `Usage:
  protoradar check-breaking <module> --path <buf-workspace> [--against latest|<version>] [--target-ref <ref>] [--report-file <path>]

Exit codes:
  0 no breaking changes
  1 breaking changes found
  2 input, auth, network, server, config, or internal error
`
	case "gitlab":
		return `Usage:
  protoradar gitlab mr-check [flags]
`
	case "gitlab mr-check":
		return `Usage:
  protoradar gitlab mr-check --module <module> --path <buf-workspace> --gitlab-base-url <url> --project-id <id> --merge-request-iid <iid> --commit-sha <sha> --gitlab-token <token>

In GitLab CI, PROTORADAR_SERVER_URL, PROTORADAR_TOKEN, PROTORADAR_GITLAB_TOKEN, and GITLAB_TOKEN are supported.
Exit code 1 means breaking changes were found, not a tool failure.
--governance-actor is deprecated and only works when server actor override is enabled. Omit it to use the authenticated ProtoRadar principal.
`
	case "runtime":
		return `Usage:
  protoradar runtime report [--from-file <path>] [--module <module@version>]...
`
	case "edition":
		return `Usage:
  protoradar edition
`
	case "runtime report":
		return `Usage:
  protoradar runtime report --service <service> --environment <env> --git-commit <sha> --build-version <version> --module <module@version>
  protoradar runtime report --from-file protoradar-runtime.yaml

Drift statuses such as behind_latest or unknown_version are successful inventory results and exit 0.
`
	case "approvals":
		return `Usage:
  protoradar approvals <request|status|approve|reject> [flags]
`
	case "approvals request":
		return `Usage:
  protoradar approvals request --report-id <breaking_report_id> [--actor <actor>]

--actor is deprecated and only works when server actor override is enabled. Omit it to use the authenticated principal.
`
	case "approvals status":
		return `Usage:
  protoradar approvals status --report-id <breaking_report_id>
`
	case "approvals approve":
		return `Usage:
  protoradar approvals approve <requirement_id> --request-id <approval_request_id> [--actor <actor>] [--comment <comment>]

--actor is deprecated and only works when server actor override is enabled. Omit it to use the authenticated principal.
`
	case "approvals reject":
		return `Usage:
  protoradar approvals reject <requirement_id> --request-id <approval_request_id> [--actor <actor>] [--comment <comment>]

--actor is deprecated and only works when server actor override is enabled. Omit it to use the authenticated principal.
`
	default:
		return `ProtoRadar CLI

Usage:
  protoradar <command> [flags]

Commands:
  login                  Store server URL and API token.
  version                Print version, commit, and build date.
  edition                Print server edition and capabilities.
  module create          Create a protobuf module.
  module list            List modules. Alias: protoradar list.
  module link-gitlab     Link a module to a GitLab project.
  module dependencies    Show direct upstream/downstream protobuf dependencies.
  module affected        Show direct downstream modules affected by a module.
  module owners          Manage module owners and maintainers.
  module version         Manage module version lifecycle.
  push                   Publish a Buf-compatible module version.
  pull                   Download a published source artifact.
  check-breaking         Run a server-side breaking-change check.
  gitlab mr-check        Run GitLab MR bot check/comment/status flow.
  runtime report         Report deployed module versions.
  approvals              Request, inspect, approve, or reject governance approvals.

Authentication:
  Use protoradar login, or set PROTORADAR_SERVER_URL and PROTORADAR_TOKEN in CI.

Exit codes:
  0 success or no breaking changes
  1 breaking changes found by check-breaking or gitlab mr-check
  2 invalid input, auth, network, server, config, or internal error

Examples:
  protoradar login --server http://localhost:8080 --token <token>
  protoradar edition
  protoradar module create user-api --description "User API contracts"
  protoradar push user-api --version v1.0.0 --path examples/repos/user-api
  protoradar check-breaking user-api --path . --against latest
`
	}
}
