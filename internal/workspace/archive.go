package workspace

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ExtractOptions struct {
	MaxUncompressedSizeBytes int64
}

func ExtractTarGzSafe(ctx context.Context, reader io.Reader, dst string, options ExtractOptions) error {
	if options.MaxUncompressedSizeBytes <= 0 {
		return fmt.Errorf("max uncompressed size bytes must be positive")
	}
	if strings.TrimSpace(dst) == "" {
		return fmt.Errorf("destination directory is required")
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	var totalSize int64
	entries := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			if entries == 0 {
				return fmt.Errorf("archive is empty")
			}
			return nil
		}
		if err != nil {
			return err
		}
		entries++

		rawName := header.Name
		if header.Typeflag == tar.TypeDir {
			rawName = strings.TrimRight(rawName, "/")
		}
		name, err := safeArchivePath(rawName)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(name))
		if !insideDir(dst, target) {
			return fmt.Errorf("unsafe archive path %q escapes destination", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 {
				return fmt.Errorf("unsafe archive path %q has negative size", header.Name)
			}
			if totalSize+header.Size > options.MaxUncompressedSizeBytes {
				return fmt.Errorf("archive exceeds max uncompressed size")
			}
			if err := writeRegularFile(target, tarReader, header.Size); err != nil {
				return err
			}
			totalSize += header.Size
		case tar.TypeSymlink:
			return fmt.Errorf("unsafe archive path %q is a symlink", header.Name)
		case tar.TypeLink:
			return fmt.Errorf("unsafe archive path %q is a hardlink", header.Name)
		default:
			return fmt.Errorf("unsupported archive entry %q", header.Name)
		}
	}
}

func writeRegularFile(target string, reader io.Reader, size int64) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("unsafe archive path %q targets a symlink", target)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.CopyN(file, reader, size)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func safeArchivePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("unsafe archive path is empty")
	}
	if strings.Contains(path, "\\") {
		return "", fmt.Errorf("unsafe archive path %q contains backslash", path)
	}
	if strings.HasPrefix(path, "/") || filepath.IsAbs(path) {
		return "", fmt.Errorf("unsafe archive path %q is absolute", path)
	}

	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe archive path %q uses path traversal", path)
	}
	if clean != path {
		return "", fmt.Errorf("unsafe archive path %q is not clean", path)
	}
	return clean, nil
}

func insideDir(root string, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
