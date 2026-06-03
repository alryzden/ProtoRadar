package storage

import (
	"fmt"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func BuildArtifactKey(module domain.ModuleName, version domain.Version, checksumSHA256 string) string {
	return BuildSourceArchiveKey(module, version, checksumSHA256)
}

func BuildSourceArchiveKey(module domain.ModuleName, version domain.Version, checksumSHA256 string) string {
	checksum := strings.TrimSpace(checksumSHA256)
	return fmt.Sprintf("modules/%s/versions/%s/source/sha256-%s.tar.gz", module.String(), version.String(), checksum)
}

func BuildBufImageKey(module domain.ModuleName, version domain.Version, checksumSHA256 string) string {
	checksum := strings.TrimSpace(checksumSHA256)
	return fmt.Sprintf("modules/%s/versions/%s/buf-image/sha256-%s.binpb", module.String(), version.String(), checksum)
}
