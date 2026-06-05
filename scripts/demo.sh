#!/usr/bin/env bash
set -euo pipefail

SERVER_URL="${PROTORADAR_SERVER_URL:-http://localhost:8080}"
BOOTSTRAP_TOKEN="${PROTORADAR_AUTH_BOOTSTRAP_TOKEN:-local-bootstrap-token}"
CLI="${PROTORADAR_CLI:-}"
VERSION="${PROTORADAR_DEMO_VERSION:-v1.0.0}"

require_command() {
	if ! command -v "$1" >/dev/null 2>&1; then
		echo "missing required command: $1" >&2
		exit 2
	fi
}

find_cli() {
	if [ -n "$CLI" ]; then
		printf '%s\n' "$CLI"
		return
	fi
	if [ -x ./bin/protoradar ]; then
		printf '%s\n' ./bin/protoradar
		return
	fi
	if command -v protoradar >/dev/null 2>&1; then
		command -v protoradar
		return
	fi
	echo "protoradar CLI not found; run make build or set PROTORADAR_CLI" >&2
	exit 2
}

extract_token() {
	sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
}

wait_for_ready() {
	attempts=60
	while [ "$attempts" -gt 0 ]; do
		if curl -fsS "$SERVER_URL/readyz" >/dev/null 2>&1; then
			return 0
		fi
		attempts=$((attempts - 1))
		sleep 1
	done
	echo "timed out waiting for $SERVER_URL/readyz" >&2
	exit 2
}

ensure_token() {
	if [ -n "${PROTORADAR_TOKEN:-}" ]; then
		return
	fi
	response="$(curl -fsS \
		-H "Authorization: Bearer $BOOTSTRAP_TOKEN" \
		-H "Content-Type: application/json" \
		-d "{\"name\":\"local-demo-$(date +%Y%m%d%H%M%S)\"}" \
		"$SERVER_URL/api/v1/tokens")"
	PROTORADAR_TOKEN="$(printf '%s' "$response" | extract_token)"
	if [ -z "$PROTORADAR_TOKEN" ]; then
		echo "could not extract API token from token response" >&2
		exit 2
	fi
	export PROTORADAR_TOKEN
}

run_allowing_existing() {
	description="$1"
	shift
	set +e
	output="$("$@" 2>&1)"
	status=$?
	set -e
	if [ "$status" -eq 0 ]; then
		printf '%s\n' "$output"
		return 0
	fi
	case "$output" in
		*"already exists"*|*"version"*"already exists"*)
			echo "$description already exists; continuing."
			return 0
			;;
	esac
	printf '%s\n' "$output" >&2
	return "$status"
}

require_command curl
CLI_PATH="$(find_cli)"
wait_for_ready
ensure_token
export PROTORADAR_SERVER_URL="$SERVER_URL"

run_allowing_existing "module user-api" "$CLI_PATH" module create user-api \
	--description "User service protobuf contracts" \
	--repository-url "https://gitlab.example.com/platform/user-api"
run_allowing_existing "module billing-api" "$CLI_PATH" module create billing-api \
	--description "Billing service protobuf contracts" \
	--repository-url "https://gitlab.example.com/platform/billing-api"
run_allowing_existing "module notification-api" "$CLI_PATH" module create notification-api \
	--description "Notification service protobuf contracts" \
	--repository-url "https://gitlab.example.com/platform/notification-api"

run_allowing_existing "user-api $VERSION" "$CLI_PATH" push user-api --version "$VERSION" --path examples/repos/user-api
run_allowing_existing "billing-api $VERSION" "$CLI_PATH" push billing-api --version "$VERSION" --path examples/repos/billing-api
run_allowing_existing "notification-api $VERSION" "$CLI_PATH" push notification-api --version "$VERSION" --path examples/repos/notification-api

"$CLI_PATH" runtime report --from-file examples/repos/billing-api/protoradar-runtime.yaml
"$CLI_PATH" runtime report --from-file examples/repos/notification-api/protoradar-runtime.yaml
"$CLI_PATH" module dependencies user-api
"$CLI_PATH" module affected user-api

echo "Demo completed."
echo "Web UI: $SERVER_URL/ui"
echo "Dependency graph: $SERVER_URL/ui/modules/user-api/dependencies"
