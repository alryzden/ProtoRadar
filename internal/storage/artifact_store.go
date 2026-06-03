package storage

import (
	"context"
	"io"
)

type ArtifactObject struct {
	Key         string
	ContentType string
	SizeBytes   int64
	Body        io.ReadCloser
}

type ArtifactStore interface {
	Put(ctx context.Context, key string, body io.Reader, sizeBytes int64) (ArtifactObject, error)
	Get(ctx context.Context, key string) (ArtifactObject, error)
	Delete(ctx context.Context, key string) error
}
