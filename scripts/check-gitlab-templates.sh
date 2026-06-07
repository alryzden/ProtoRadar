#!/usr/bin/env bash
set -euo pipefail

templates=(
	"examples/gitlab/protoradar-breaking-check.yml"
	"examples/gitlab/protoradar-mr-check.yml"
	"examples/gitlab/protoradar-publish.yml"
	"examples/gitlab/protoradar-runtime-report.yml"
)

failed=0
for template in "${templates[@]}"; do
	if ! grep -q 'PROTORADAR_CLI_IMAGE' "$template"; then
		echo "$template: missing PROTORADAR_CLI_IMAGE" >&2
		failed=1
	fi
	if ! grep -q 'image: "$PROTORADAR_CLI_IMAGE"' "$template"; then
		echo "$template: image must use \"\$PROTORADAR_CLI_IMAGE\"" >&2
		failed=1
	fi
	for required in PROTORADAR_SERVER_URL PROTORADAR_TOKEN PROTORADAR_MODULE; do
		if ! grep -q "$required" "$template"; then
			echo "$template: missing $required" >&2
			failed=1
		fi
	done
done

for forbidden in \
	'echo[[:space:]]+\$PROTORADAR_TOKEN' \
	'echo[[:space:]]+"\$PROTORADAR_TOKEN"' \
	'echo[[:space:]]+\$\{PROTORADAR_TOKEN\}' \
	'echo[[:space:]]+"\$\{PROTORADAR_TOKEN\}"' \
	'echo[[:space:]]+\$GITLAB_TOKEN' \
	'echo[[:space:]]+"\$GITLAB_TOKEN"' \
	'echo[[:space:]]+\$\{GITLAB_TOKEN\}' \
	'echo[[:space:]]+"\$\{GITLAB_TOKEN\}"' \
	'echo[[:space:]]+\$PROTORADAR_GITLAB_TOKEN' \
	'echo[[:space:]]+"\$PROTORADAR_GITLAB_TOKEN"' \
	'echo[[:space:]]+\$\{PROTORADAR_GITLAB_TOKEN\}' \
	'echo[[:space:]]+"\$\{PROTORADAR_GITLAB_TOKEN\}"' \
	'set[[:space:]]+-x' \
	'printenv' \
	'^[[:space:]]*env([[:space:]]|$)' \
	'--gitlab-token' \
	'--api-token' \
	'--token'; do
	if grep -En -- "$forbidden" "${templates[@]}"; then
		echo "forbidden pattern found: $forbidden" >&2
		failed=1
	fi
done

if [ "$failed" -ne 0 ]; then
	exit 1
fi

echo "GitLab template checks passed"
