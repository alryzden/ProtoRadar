package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCreateArtifactIncludesBufConfigLockAndProtoFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	writeFile(t, filepath.Join(root, "buf.lock"), "deps: []\n")
	writeFile(t, filepath.Join(root, "user.proto"), "syntax = \"proto3\";")
	writeFile(t, filepath.Join(root, "README.md"), "docs")
	writeFile(t, filepath.Join(root, "nested", "billing.proto"), "syntax = \"proto3\";")

	artifact, err := createArtifact(root)
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	names := archiveNames(t, artifact.Body)
	if !slices.Contains(names, "buf.yaml") {
		t.Fatalf("missing buf.yaml: %#v", names)
	}
	if !slices.Contains(names, "buf.lock") {
		t.Fatalf("missing buf.lock: %#v", names)
	}
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
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	writeFile(t, filepath.Join(root, "a", "b", "service.proto"), "syntax = \"proto3\";")

	artifact, err := createArtifact(root)
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	names := archiveNames(t, artifact.Body)
	if !slices.Contains(names, "a/b/service.proto") {
		t.Fatalf("names = %#v", names)
	}
}

func TestCreateArtifactRequiresBufYAML(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "user.proto"), "syntax = \"proto3\";")

	_, err := createArtifact(root)
	if err == nil || !strings.Contains(err.Error(), "buf.yaml") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateArtifactRejectsEmptyProtoDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	writeFile(t, filepath.Join(root, "README.md"), "docs")

	_, err := createArtifact(root)
	if err == nil || !strings.Contains(err.Error(), "no .proto") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateArtifactExcludesIrrelevantDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	writeFile(t, filepath.Join(root, "user.proto"), "syntax = \"proto3\";")
	for _, dir := range []string{".git", "node_modules", "tmp", "dist", "build", "generated", "vendor"} {
		writeFile(t, filepath.Join(root, dir, "ignored.proto"), "syntax = \"proto3\";")
	}

	artifact, err := createArtifact(root)
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	names := archiveNames(t, artifact.Body)
	for _, name := range names {
		if strings.Contains(name, "ignored.proto") {
			t.Fatalf("included excluded directory file: %#v", names)
		}
	}
}

func TestCreateArtifactHandlesManyFilesWithoutChangingArchiveContents(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	for i := 0; i < 200; i++ {
		writeFile(t, filepath.Join(root, fmt.Sprintf("service_%03d.proto", i)), "syntax = \"proto3\";")
	}

	artifact, err := createArtifact(root)
	if err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	if artifact.FileCount != 200 {
		t.Fatalf("file count = %d, want 200", artifact.FileCount)
	}

	names := archiveNames(t, artifact.Body)
	if !slices.Contains(names, "buf.yaml") {
		t.Fatalf("missing buf.yaml: %#v", names)
	}
	for i := 0; i < 200; i++ {
		name := fmt.Sprintf("service_%03d.proto", i)
		if !slices.Contains(names, name) {
			t.Fatalf("missing %s: %#v", name, names)
		}
	}
}

func TestCreateArtifactRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	writeFile(t, filepath.Join(root, "user.proto"), "syntax = \"proto3\";")
	if err := os.Symlink(filepath.Join(root, "user.proto"), filepath.Join(root, "link.proto")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := createArtifact(root)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %v", err)
	}
}

func TestCopyAndCloseClosesReaderWhenCopyFails(t *testing.T) {
	copyErr := errors.New("copy failed")
	reader := &errorReadCloser{readErr: copyErr}

	err := copyAndClose(io.Discard, reader)
	if !errors.Is(err, copyErr) {
		t.Fatalf("error = %v, want copy error", err)
	}
	if !reader.closed {
		t.Fatal("reader was not closed")
	}
}

func TestCopyAndCloseSurfacesCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	reader := &errorReadCloser{body: []byte("artifact"), closeErr: closeErr}

	err := copyAndClose(io.Discard, reader)
	if !errors.Is(err, closeErr) {
		t.Fatalf("error = %v, want close error", err)
	}
	if !reader.closed {
		t.Fatal("reader was not closed")
	}
}

func TestCopyAndCloseJoinsCopyAndCloseErrors(t *testing.T) {
	copyErr := errors.New("copy failed")
	closeErr := errors.New("close failed")
	reader := &errorReadCloser{readErr: copyErr, closeErr: closeErr}

	err := copyAndClose(io.Discard, reader)
	if !errors.Is(err, copyErr) {
		t.Fatalf("error = %v, want copy error", err)
	}
	if !errors.Is(err, closeErr) {
		t.Fatalf("error = %v, want close error", err)
	}
	if !reader.closed {
		t.Fatal("reader was not closed")
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

type errorReadCloser struct {
	body     []byte
	readErr  error
	closeErr error
	closed   bool
}

func (reader *errorReadCloser) Read(p []byte) (int, error) {
	if reader.readErr != nil {
		err := reader.readErr
		reader.readErr = nil
		return 0, err
	}
	if len(reader.body) == 0 {
		return 0, io.EOF
	}
	n := copy(p, reader.body)
	reader.body = reader.body[n:]
	return n, nil
}

func (reader *errorReadCloser) Close() error {
	reader.closed = true
	return reader.closeErr
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
