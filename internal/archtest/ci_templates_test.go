package archtest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDockerfileDefinesUsableCLIImage(t *testing.T) {
	repoRoot := findRepoRoot(t)
	dockerfile := readRepoFile(t, repoRoot, "Dockerfile")
	dockerignore := readRepoFile(t, repoRoot, ".dockerignore")

	assertContains(t, "Dockerfile", dockerfile, "FROM golang:1.26-alpine AS build")
	assertContains(t, "Dockerfile", dockerfile, "CGO_ENABLED=0 go build")
	assertContains(t, "Dockerfile", dockerfile, "-o /out/protoradar ./cmd/protoradar")

	cliStage := dockerfileStage(t, dockerfile, "cli")
	assertContains(t, "Dockerfile cli stage", cliStage, "COPY --from=build /out/protoradar /usr/local/bin/protoradar")
	if !strings.Contains(cliStage, `ENTRYPOINT ["protoradar"]`) && !strings.Contains(cliStage, `CMD ["protoradar"]`) {
		t.Fatalf("Dockerfile cli stage must define ENTRYPOINT or CMD for protoradar")
	}

	forbidden := []string{
		"COPY .env",
		"COPY .env.",
		"COPY secrets",
		"COPY secret",
		"COPY *secret*",
		"COPY *token*",
		"ARG PROTORADAR_TOKEN",
		"ARG GITLAB_TOKEN",
		"ARG PROTORADAR_GITLAB_TOKEN",
		"ENV PROTORADAR_TOKEN",
		"ENV GITLAB_TOKEN",
		"ENV PROTORADAR_GITLAB_TOKEN",
	}
	for _, pattern := range forbidden {
		if strings.Contains(dockerfile, pattern) {
			t.Fatalf("Dockerfile must not copy or define obvious secrets; found %q", pattern)
		}
	}
	for _, ignored := range []string{".env", ".env.*", ".git"} {
		assertContains(t, ".dockerignore", dockerignore, ignored)
	}
}

func TestMakefileDefinesCLIImageTargets(t *testing.T) {
	repoRoot := findRepoRoot(t)
	makefile := readRepoFile(t, repoRoot, "Makefile")

	assertMakeTarget(t, makefile, "docker-build-cli")
	assertMakeTarget(t, makefile, "docker-smoke-cli")
	assertContains(t, "Makefile", makefile, "--target cli")
	assertContains(t, "Makefile", makefile, "scripts/smoke-cli-image.sh")
}

func TestGitLabTemplatesUseCLIImageAndDoNotLeakSecrets(t *testing.T) {
	repoRoot := findRepoRoot(t)
	templates := gitLabTemplateFiles(t, repoRoot)
	if len(templates) == 0 {
		t.Fatal("no GitLab template files found")
	}

	for _, template := range templates {
		content := readRepoFile(t, repoRoot, template)
		if !strings.Contains(content, "protoradar ") {
			continue
		}

		if strings.Contains(content, "image:") || strings.Contains(content, "protoradar ") {
			assertContains(t, template, content, "PROTORADAR_CLI_IMAGE")
		}
		if strings.Contains(content, "image:") {
			assertContains(t, template, content, `image: "$PROTORADAR_CLI_IMAGE"`)
		}
		assertContains(t, template, content, "PROTORADAR_SERVER_URL")
		assertContains(t, template, content, "PROTORADAR_TOKEN")
		if strings.Contains(content, "check-breaking") || strings.Contains(content, "mr-check") || strings.Contains(content, "protoradar push") || strings.Contains(content, "runtime report") {
			assertContains(t, template, content, "PROTORADAR_MODULE")
		}

		assertNoForbiddenTemplateContent(t, template, content)
		if strings.Contains(content, "gitlab mr-check") {
			assertContains(t, template, content, "PROTORADAR_GITLAB_TOKEN")
			if strings.Contains(content, "--gitlab-token") {
				t.Fatalf("%s must use env-based GitLab token auth, not --gitlab-token", template)
			}
		}
		if strings.Contains(content, "--token") || strings.Contains(content, "--api-token") {
			t.Fatalf("%s must use env-based ProtoRadar token auth, not token CLI arguments", template)
		}
	}
}

func TestGitLabCIDocsDescribeCLIImageAndRequiredVariables(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docs := map[string]string{
		"docs/gitlab-ci.md":         readRepoFile(t, repoRoot, "docs/gitlab-ci.md"),
		"docs/gitlab-mr-bot.md":     readRepoFile(t, repoRoot, "docs/gitlab-mr-bot.md"),
		"examples/gitlab/README.md": readRepoFile(t, repoRoot, "examples/gitlab/README.md"),
	}

	for path, content := range docs {
		assertContains(t, path, content, "PROTORADAR_CLI_IMAGE")
		assertContains(t, path, content, "docker-build-cli")
		assertContains(t, path, content, "PROTORADAR_SERVER_URL")
		assertContains(t, path, content, "PROTORADAR_TOKEN")
		assertContains(t, path, content, "PROTORADAR_MODULE")
	}
}

func readRepoFile(t *testing.T, repoRoot string, relativePath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return string(data)
}

func dockerfileStage(t *testing.T, dockerfile string, stageName string) string {
	t.Helper()
	stageHeader := regexp.MustCompile(`(?m)^FROM\s+\S+\s+AS\s+` + regexp.QuoteMeta(stageName) + `$`)
	match := stageHeader.FindStringIndex(dockerfile)
	if match == nil {
		t.Fatalf("Dockerfile missing %q stage", stageName)
	}
	stage := dockerfile[match[0]:]
	if next := regexp.MustCompile(`(?m)\nFROM\s+`).FindStringIndex(stage[len("\n"):]); next != nil {
		stage = stage[:len("\n")+next[0]]
	}
	return stage
}

func assertMakeTarget(t *testing.T, makefile string, target string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(target) + `:`)
	if !pattern.MatchString(makefile) {
		t.Fatalf("Makefile missing %s target", target)
	}
}

func gitLabTemplateFiles(t *testing.T, repoRoot string) []string {
	t.Helper()
	patterns := []string{
		"examples/gitlab/*.yml",
		"examples/gitlab/*/.gitlab-ci.yml",
	}
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(repoRoot, pattern))
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		for _, match := range matches {
			relativePath, err := filepath.Rel(repoRoot, match)
			if err != nil {
				t.Fatalf("relative path for %s: %v", match, err)
			}
			files = append(files, relativePath)
		}
	}
	return files
}

func assertNoForbiddenTemplateContent(t *testing.T, path string, content string) {
	t.Helper()
	forbiddenSubstrings := []string{
		`echo $PROTORADAR_TOKEN`,
		`echo "$PROTORADAR_TOKEN"`,
		`echo ${PROTORADAR_TOKEN}`,
		`echo "${PROTORADAR_TOKEN}"`,
		`echo $GITLAB_TOKEN`,
		`echo "$GITLAB_TOKEN"`,
		`echo ${GITLAB_TOKEN}`,
		`echo "${GITLAB_TOKEN}"`,
		`echo $PROTORADAR_GITLAB_TOKEN`,
		`echo "$PROTORADAR_GITLAB_TOKEN"`,
		`echo ${PROTORADAR_GITLAB_TOKEN}`,
		`echo "${PROTORADAR_GITLAB_TOKEN}"`,
		"set -x",
		"printenv",
	}
	for _, forbidden := range forbiddenSubstrings {
		if strings.Contains(content, forbidden) {
			t.Fatalf("%s contains forbidden secret-unsafe pattern %q", path, forbidden)
		}
	}

	for lineNumber, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "env" || strings.HasPrefix(trimmed, "env ") || strings.HasPrefix(trimmed, "- env ") {
			t.Fatalf("%s:%d must not print or run env wholesale", path, lineNumber+1)
		}
	}
}

func assertContains(t *testing.T, path string, content string, want string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Fatalf("%s missing %q", path, want)
	}
}
