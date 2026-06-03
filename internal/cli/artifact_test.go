package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCreateArtifactIncludesOnlyProtoFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "user.proto"), "syntax = \"proto3\";")
	writeFile(t, filepath.Join(root, "README.md"), "docs")
	writeFile(t, filepath.Join(root, "nested", "billing.proto"), "syntax = \"proto3\";")

	artifact, err := createArtifact(root)
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	names := archiveNames(t, artifact.Body)
	if !slices.Contains(names, "user.proto") {
		t.Fatalf("missing user.proto: %#v", names)
	}
	if !slices.Contains(names, "nested/billing.proto") {
		t.Fatalf("missing nested proto: %#v", names)
	}
	if slices.Contains(names, "README.md") {
		t.Fatalf("included non-proto file: %#v", names)
	}
}

func TestCreateArtifactPreservesRelativePaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a", "b", "service.proto"), "syntax = \"proto3\";")

	artifact, err := createArtifact(root)
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	names := archiveNames(t, artifact.Body)
	if len(names) != 1 || names[0] != "a/b/service.proto" {
		t.Fatalf("names = %#v", names)
	}
}

func TestCreateArtifactRejectsEmptyProtoDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "README.md"), "docs")

	_, err := createArtifact(root)
	if err == nil || !strings.Contains(err.Error(), "no .proto") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractArtifactBlocksPathTraversal(t *testing.T) {
	body := buildArchive(t, "../evil.proto", "syntax = \"proto3\";")

	err := extractArtifact(bytes.NewReader(body), t.TempDir(), false)
	if err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractArtifactRefusesNonEmptyOutputWithoutForce(t *testing.T) {
	output := t.TempDir()
	writeFile(t, filepath.Join(output, "existing.txt"), "existing")
	body := buildArchive(t, "user.proto", "syntax = \"proto3\";")

	err := extractArtifact(bytes.NewReader(body), output, false)
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("error = %v", err)
	}

	if err := extractArtifact(bytes.NewReader(body), output, true); err != nil {
		t.Fatalf("extract with force: %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "user.proto")); err != nil {
		t.Fatalf("stat extracted file: %v", err)
	}
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func archiveNames(t *testing.T, body []byte) []string {
	t.Helper()
	gzipReader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	var names []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		names = append(names, header.Name)
	}
}

func buildArchive(t *testing.T, name string, body string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(body))}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tarWriter.Write([]byte(body)); err != nil {
		t.Fatalf("write body: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buffer.Bytes()
}
