package storage

import (
	"fmt"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func BuildArtifactKey(module domain.ModuleName, version domain.Version, checksumSHA256 string) string {
	checksum := strings.TrimSpace(checksumSHA256)
	return fmt.Sprintf("modules/%s/versions/%s/sha256-%s.tar.gz", module.String(), version.String(), checksum)
}
