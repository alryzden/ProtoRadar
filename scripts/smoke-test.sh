#!/usr/bin/env bash
set -euo pipefail

SERVER_URL="${PROTORADAR_SERVER_URL:-http://localhost:8080}"
BOOTSTRAP_TOKEN="${PROTORADAR_AUTH_BOOTSTRAP_TOKEN:-local-bootstrap-token}"
CLI="${PROTORADAR_CLI:-}"
RUN_ID="${PROTORADAR_SMOKE_RUN_ID:-$(date +%Y%m%d%H%M%S)}"
USER_MODULE="pr-smoke-user-$RUN_ID"
BILLING_MODULE="pr-smoke-billing-$RUN_ID"
VERSION="v1.0.0"

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

wait_for_endpoint() {
	path="$1"
	attempts="${2:-60}"
	while [ "$attempts" -gt 0 ]; do
		if curl -fsS "$SERVER_URL$path" >/dev/null 2>&1; then
			return 0
		fi
		attempts=$((attempts - 1))
		sleep 1
	done
	echo "timed out waiting for $SERVER_URL$path" >&2
	exit 2
}

ensure_token() {
	if [ -n "${PROTORADAR_TOKEN:-}" ]; then
		return
	fi
	response="$(curl -fsS \
		-H "Authorization: Bearer $BOOTSTRAP_TOKEN" \
		-H "Content-Type: application/json" \
		-d "{\"name\":\"smoke-test-$RUN_ID\"}" \
		"$SERVER_URL/api/v1/tokens")"
	PROTORADAR_TOKEN="$(printf '%s' "$response" | extract_token)"
	if [ -z "$PROTORADAR_TOKEN" ]; then
		echo "could not extract API token from token response" >&2
		exit 2
	fi
	export PROTORADAR_TOKEN
}

require_command curl
CLI_PATH="$(find_cli)"

wait_for_endpoint /healthz 60
health="$(curl -fsS "$SERVER_URL/healthz")"
case "$health" in
	*'"status":"ok"'*) ;;
	*) echo "/healthz did not return ok: $health" >&2; exit 2 ;;
esac

wait_for_endpoint /readyz 60
ready="$(curl -fsS "$SERVER_URL/readyz")"
case "$ready" in
	*'"status":"ok"'*) ;;
	*) echo "/readyz did not return ok: $ready" >&2; exit 2 ;;
esac

metrics_headers="$(mktemp)"
metrics_body="$(curl -fsS -D "$metrics_headers" "$SERVER_URL/metrics")"
if ! grep -qi 'content-type: text/plain' "$metrics_headers"; then
	echo "/metrics did not return Prometheus text content type" >&2
	rm -f "$metrics_headers"
	exit 2
fi
rm -f "$metrics_headers"
case "$metrics_body" in
	*protoradar_http_requests_total*) ;;
	*) echo "metrics response is missing protoradar_http_requests_total" >&2; exit 2 ;;
esac

ensure_token
export PROTORADAR_SERVER_URL="$SERVER_URL"

"$CLI_PATH" module create "$USER_MODULE" --description "Smoke test user API"
"$CLI_PATH" module create "$BILLING_MODULE" --description "Smoke test billing API"
"$CLI_PATH" push "$USER_MODULE" --version "$VERSION" --path examples/repos/user-api
"$CLI_PATH" push "$BILLING_MODULE" --version "$VERSION" --path examples/repos/billing-api

dependencies="$("$CLI_PATH" module dependencies "$USER_MODULE")"
case "$dependencies" in
	*"$BILLING_MODULE"*) ;;
	*) echo "dependency graph did not show $BILLING_MODULE depending on $USER_MODULE" >&2; printf '%s\n' "$dependencies" >&2; exit 2 ;;
esac

affected="$("$CLI_PATH" module affected "$USER_MODULE")"
case "$affected" in
	*"$BILLING_MODULE"*) ;;
	*) echo "affected modules did not include $BILLING_MODULE" >&2; printf '%s\n' "$affected" >&2; exit 2 ;;
esac

"$CLI_PATH" check-breaking "$USER_MODULE" --path examples/repos/user-api --against "$VERSION" >/dev/null
"$CLI_PATH" runtime report \
	--service "smoke-billing-service-$RUN_ID" \
	--environment production \
	--git-commit "smoke-$RUN_ID" \
	--build-version "$VERSION" \
	--module "$USER_MODULE@$VERSION" \
	--module "$BILLING_MODULE@$VERSION" >/dev/null

curl -fsS "$SERVER_URL/ui/modules" >/dev/null

echo "Smoke test passed for $SERVER_URL using modules $USER_MODULE and $BILLING_MODULE"
