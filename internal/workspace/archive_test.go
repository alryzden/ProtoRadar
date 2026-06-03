package workspace

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractTarGzSafeAcceptsNormalProtoTree(t *testing.T) {
	body := buildTarGz(t,
		tarEntry{name: "buf.yaml", body: []byte("version: v2\n")},
		tarEntry{name: "proto/user/v1/user.proto", body: []byte("syntax = \"proto3\";")},
		tarEntry{name: "proto/billing/v1/billing.proto", body: []byte("syntax = \"proto3\";")},
	)
	dst := t.TempDir()

	if err := ExtractTarGzSafe(context.Background(), bytes.NewReader(body), dst, ExtractOptions{MaxUncompressedSizeBytes: 1024}); err != nil {
		t.Fatalf("extract: %v", err)
	}

	assertFile(t, filepath.Join(dst, "buf.yaml"), "version: v2\n")
	assertFile(t, filepath.Join(dst, "proto", "user", "v1", "user.proto"), "syntax = \"proto3\";")
	assertFile(t, filepath.Join(dst, "proto", "billing", "v1", "billing.proto"), "syntax = \"proto3\";")
}

func TestExtractTarGzSafePreservesRelativePaths(t *testing.T) {
	body := buildTarGz(t, tarEntry{name: "a/b/c/service.proto", body: []byte("proto")})
	dst := t.TempDir()

	if err := ExtractTarGzSafe(context.Background(), bytes.NewReader(body), dst, ExtractOptions{MaxUncompressedSizeBytes: 1024}); err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertFile(t, filepath.Join(dst, "a", "b", "c", "service.proto"), "proto")
}

func TestExtractTarGzSafeRejectsPathTraversal(t *testing.T) {
	body := buildTarGz(t, tarEntry{name: "../evil.proto", body: []byte("evil")})

	err := ExtractTarGzSafe(context.Background(), bytes.NewReader(body), t.TempDir(), ExtractOptions{MaxUncompressedSizeBytes: 1024})
	assertErrorContains(t, err, "path traversal")
}

func TestExtractTarGzSafeRejectsAbsolutePath(t *testing.T) {
	body := buildTarGz(t, tarEntry{name: "/evil.proto", body: []byte("evil")})

	err := ExtractTarGzSafe(context.Background(), bytes.NewReader(body), t.TempDir(), ExtractOptions{MaxUncompressedSizeBytes: 1024})
	assertErrorContains(t, err, "absolute")
}

func TestExtractTarGzSafeRejectsSymlink(t *testing.T) {
	body := buildTarGz(t, tarEntry{name: "proto/link.proto", typeflag: tar.TypeSymlink, linkname: "user.proto"})

	err := ExtractTarGzSafe(context.Background(), bytes.NewReader(body), t.TempDir(), ExtractOptions{MaxUncompressedSizeBytes: 1024})
	assertErrorContains(t, err, "symlink")
}

func TestExtractTarGzSafeRejectsHardlink(t *testing.T) {
	body := buildTarGz(t, tarEntry{name: "proto/link.proto", typeflag: tar.TypeLink, linkname: "user.proto"})

	err := ExtractTarGzSafe(context.Background(), bytes.NewReader(body), t.TempDir(), ExtractOptions{MaxUncompressedSizeBytes: 1024})
	assertErrorContains(t, err, "hardlink")
}

func TestExtractTarGzSafeRejectsArchiveExceedingMaxUncompressedSize(t *testing.T) {
	body := buildTarGz(t,
		tarEntry{name: "one.proto", body: []byte("12345")},
		tarEntry{name: "two.proto", body: []byte("67890")},
	)

	err := ExtractTarGzSafe(context.Background(), bytes.NewReader(body), t.TempDir(), ExtractOptions{MaxUncompressedSizeBytes: 9})
	assertErrorContains(t, err, "max uncompressed size")
}

func TestExtractTarGzSafeRejectsSuspiciousFileName(t *testing.T) {
	body := buildTarGz(t, tarEntry{name: `proto\evil.proto`, body: []byte("evil")})

	err := ExtractTarGzSafe(context.Background(), bytes.NewReader(body), t.TempDir(), ExtractOptions{MaxUncompressedSizeBytes: 1024})
	assertErrorContains(t, err, "backslash")
}

func TestExtractTarGzSafeRejectsNonRegularEntry(t *testing.T) {
	body := buildTarGz(t, tarEntry{name: "proto/device.proto", typeflag: tar.TypeChar})

	err := ExtractTarGzSafe(context.Background(), bytes.NewReader(body), t.TempDir(), ExtractOptions{MaxUncompressedSizeBytes: 1024})
	assertErrorContains(t, err, "unsupported")
}

func TestExtractTarGzSafeHandlesEmptyArchiveClearly(t *testing.T) {
	body := buildTarGz(t)

	err := ExtractTarGzSafe(context.Background(), bytes.NewReader(body), t.TempDir(), ExtractOptions{MaxUncompressedSizeBytes: 1024})
	assertErrorContains(t, err, "archive is empty")
}

func TestExtractTarGzSafeAllowsDirectories(t *testing.T) {
	body := buildTarGz(t,
		tarEntry{name: "proto/", typeflag: tar.TypeDir},
		tarEntry{name: "proto/user.proto", body: []byte("proto")},
	)
	dst := t.TempDir()

	if err := ExtractTarGzSafe(context.Background(), bytes.NewReader(body), dst, ExtractOptions{MaxUncompressedSizeBytes: 1024}); err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertFile(t, filepath.Join(dst, "proto", "user.proto"), "proto")
}

type tarEntry struct {
	name     string
	body     []byte
	typeflag byte
	linkname string
}

func buildTarGz(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		header := &tar.Header{
			Name:     entry.name,
			Mode:     0o600,
			Size:     int64(len(entry.body)),
			Typeflag: typeflag,
			Linkname: entry.linkname,
		}
		if typeflag == tar.TypeDir {
			header.Mode = 0o755
			header.Size = 0
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if len(entry.body) > 0 {
			if _, err := tarWriter.Write(entry.body); err != nil {
				t.Fatalf("write body: %v", err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buffer.Bytes()
}

func assertFile(t *testing.T, path string, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	if string(body) != want {
		t.Fatalf("file %s = %q, want %q", path, string(body), want)
	}
}

func assertErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want containing %q", err, want)
	}
}
