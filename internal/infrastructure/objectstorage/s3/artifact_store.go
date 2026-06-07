package s3

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/alryzden/ProtoRadar/internal/storage"
)

const artifactContentType = "application/gzip"

type Config struct {
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

type objectClient interface {
	put(ctx context.Context, bucket string, key string, body io.Reader, sizeBytes int64, contentType string) (int64, error)
	get(ctx context.Context, bucket string, key string) (storedObject, error)
	delete(ctx context.Context, bucket string, key string) error
}

type storedObject struct {
	body        io.ReadCloser
	contentType string
	sizeBytes   int64
}

type ArtifactStore struct {
	bucket string
	client objectClient
}

func NewArtifactStore(cfg Config) (*ArtifactStore, error) {
	endpoint, secure, err := parseEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}

	bucketLookup := minio.BucketLookupAuto
	if cfg.UsePathStyle {
		bucketLookup = minio.BucketLookupPath
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       secure,
		Region:       cfg.Region,
		BucketLookup: bucketLookup,
	})
	if err != nil {
		return nil, err
	}

	return &ArtifactStore{
		bucket: cfg.Bucket,
		client: realClient{client: client},
	}, nil
}

func newArtifactStoreWithClient(bucket string, client objectClient) *ArtifactStore {
	return &ArtifactStore{
		bucket: bucket,
		client: client,
	}
}

func (store *ArtifactStore) Put(ctx context.Context, key string, body io.Reader, sizeBytes int64) (storage.ArtifactObject, error) {
	written, err := store.client.put(ctx, store.bucket, key, body, sizeBytes, artifactContentType)
	if err != nil {
		return storage.ArtifactObject{}, err
	}

	return storage.ArtifactObject{
		Key:         key,
		ContentType: artifactContentType,
		SizeBytes:   written,
	}, nil
}

func (store *ArtifactStore) Get(ctx context.Context, key string) (storage.ArtifactObject, error) {
	object, err := store.client.get(ctx, store.bucket, key)
	if err != nil {
		return storage.ArtifactObject{}, err
	}

	return storage.ArtifactObject{
		Key:         key,
		ContentType: object.contentType,
		SizeBytes:   object.sizeBytes,
		Body:        object.body,
	}, nil
}

func (store *ArtifactStore) Delete(ctx context.Context, key string) error {
	return store.client.delete(ctx, store.bucket, key)
}

func parseEndpoint(endpoint string) (string, bool, error) {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return "", false, fmt.Errorf("storage.s3.endpoint is required")
	}
	if !strings.Contains(trimmed, "://") {
		return trimmed, true, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", false, err
	}
	if parsed.Host == "" {
		return "", false, fmt.Errorf("storage.s3.endpoint must include a host")
	}

	switch parsed.Scheme {
	case "http":
		return parsed.Host, false, nil
	case "https":
		return parsed.Host, true, nil
	default:
		return "", false, fmt.Errorf("storage.s3.endpoint must use http or https")
	}
}

type realClient struct {
	client *minio.Client
}

func (client realClient) put(ctx context.Context, bucket string, key string, body io.Reader, sizeBytes int64, contentType string) (int64, error) {
	info, err := client.client.PutObject(ctx, bucket, key, body, sizeBytes, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

func (client realClient) get(ctx context.Context, bucket string, key string) (storedObject, error) {
	object, err := client.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return storedObject{}, err
	}

	info, err := object.Stat()
	if err != nil {
		closeObject(object)
		return storedObject{}, err
	}

	return storedObject{
		body:        object,
		contentType: info.ContentType,
		sizeBytes:   info.Size,
	}, nil
}

func closeObject(body io.Closer) {
	// If Stat fails, the Stat error is the actionable result; closing the
	// partially opened object is best-effort cleanup.
	_ = body.Close() //nolint:errcheck
}

func (client realClient) delete(ctx context.Context, bucket string, key string) error {
	return client.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

var _ storage.ArtifactStore = (*ArtifactStore)(nil)
