package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type ArtifactPackage struct {
	Body           []byte
	ChecksumSHA256 string
	SizeBytes      int64
	FileCount      int
}

func createArtifact(root string) (ArtifactPackage, error) {
	info, err := os.Stat(root)
	if err != nil {
		return ArtifactPackage{}, err
	}
	if !info.IsDir() {
		return ArtifactPackage{}, fmt.Errorf("%s is not a directory", root)
	}

	var body bytes.Buffer
	gzipWriter := gzip.NewWriter(&body)
	tarWriter := tar.NewWriter(gzipWriter)
	fileCount := 0

	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || filepath.Ext(path) != ".proto" {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if !safeArchivePath(name) {
			return fmt.Errorf("unsafe proto path %q", name)
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		if err := tarWriter.WriteHeader(&tar.Header{
			Name:    name,
			Mode:    0o600,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}); err != nil {
			return err
		}
		if _, err := io.Copy(tarWriter, file); err != nil {
			return err
		}
		fileCount++
		return nil
	})
	if err != nil {
		return ArtifactPackage{}, err
	}
	if fileCount == 0 {
		return ArtifactPackage{}, errors.New("no .proto files found")
	}
	if err := tarWriter.Close(); err != nil {
		return ArtifactPackage{}, err
	}
	if err := gzipWriter.Close(); err != nil {
		return ArtifactPackage{}, err
	}

	sum := sha256.Sum256(body.Bytes())
	return ArtifactPackage{
		Body:           body.Bytes(),
		ChecksumSHA256: hex.EncodeToString(sum[:]),
		SizeBytes:      int64(body.Len()),
		FileCount:      fileCount,
	}, nil
}

func extractArtifact(reader io.Reader, outputDir string, force bool) error {
	if err := ensureOutputDir(outputDir, force); err != nil {
		return err
	}

	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if !safeArchivePath(header.Name) {
			return fmt.Errorf("unsafe artifact path %q", header.Name)
		}

		target := filepath.Join(outputDir, filepath.FromSlash(header.Name))
		if !insideDir(outputDir, target) {
			return fmt.Errorf("unsafe artifact path %q", header.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(file, tarReader)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
}

func ensureOutputDir(outputDir string, force bool) error {
	info, err := os.Stat(outputDir)
	if err == nil && !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", outputDir)
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err != nil && os.IsNotExist(err) {
		return os.MkdirAll(outputDir, 0o755)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return err
	}
	if len(entries) > 0 && !force {
		return fmt.Errorf("%s is not empty; use --force to overwrite", outputDir)
	}
	return nil
}

func safeArchivePath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	return clean == path
}

func insideDir(root string, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
