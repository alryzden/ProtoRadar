#!/usr/bin/env bash
set -euo pipefail

IMAGE="${CLI_IMAGE:-${PROTORADAR_CLI_IMAGE:-protoradar-cli:local}}"
DOCKER_BIN="${DOCKER:-docker}"

if [ -z "$IMAGE" ]; then
	echo "CLI_IMAGE or PROTORADAR_CLI_IMAGE is required" >&2
	exit 2
fi

if ! command -v "$DOCKER_BIN" >/dev/null 2>&1; then
	echo "missing required command: $DOCKER_BIN" >&2
	exit 2
fi

if ! "$DOCKER_BIN" info >/dev/null 2>&1; then
	echo "Docker daemon is unavailable; start Docker and rerun this smoke test" >&2
	exit 2
fi

version_output="$($DOCKER_BIN run --rm "$IMAGE" version)"
printf '%s\n' "$version_output"
case "$version_output" in
	*"Version:"*"Commit:"*"Build date:"*) ;;
	*)
		echo "CLI image version output did not include expected build metadata" >&2
		exit 2
		;;
esac

$DOCKER_BIN run --rm "$IMAGE" --help >/dev/null

echo "CLI image smoke test passed for $IMAGE"
