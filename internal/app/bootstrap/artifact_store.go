package bootstrap

import (
	"github.com/alryzden/ProtoRadar/internal/config"
	objectstorages3 "github.com/alryzden/ProtoRadar/internal/infrastructure/objectstorage/s3"
	"github.com/alryzden/ProtoRadar/internal/storage"
)

func NewArtifactStore(cfg config.RuntimeConfig) (storage.ArtifactStore, error) {
	return objectstorages3.NewArtifactStore(objectstorages3.Config{
		Endpoint:     cfg.Storage.S3.Endpoint,
		Region:       cfg.Storage.S3.Region,
		Bucket:       cfg.Storage.S3.Bucket,
		AccessKey:    cfg.Storage.S3.AccessKey,
		SecretKey:    cfg.Storage.S3.SecretKey,
		UsePathStyle: cfg.Storage.S3.UsePathStyle,
	})
}
