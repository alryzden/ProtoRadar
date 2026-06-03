package s3

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestArtifactStorePut(t *testing.T) {
	client := &fakeClient{}
	store := newArtifactStoreWithClient("protoradar", client)

	object, err := store.Put(context.Background(), "modules/m/versions/v/sha256-a.tar.gz", bytes.NewReader([]byte("body")), 4)
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	if object.Key != "modules/m/versions/v/sha256-a.tar.gz" {
		t.Fatalf("key = %q", object.Key)
	}
	if object.ContentType != artifactContentType {
		t.Fatalf("content type = %q", object.ContentType)
	}
	if object.SizeBytes != 4 {
		t.Fatalf("size = %d", object.SizeBytes)
	}
	if client.putBucket != "protoradar" {
		t.Fatalf("bucket = %q", client.putBucket)
	}
	if client.putSize != 4 {
		t.Fatalf("put size = %d", client.putSize)
	}
}

func TestArtifactStoreGet(t *testing.T) {
	client := &fakeClient{
		getObject: storedObject{
			body:        io.NopCloser(bytes.NewReader([]byte("artifact"))),
			contentType: artifactContentType,
			sizeBytes:   8,
		},
	}
	store := newArtifactStoreWithClient("protoradar", client)

	object, err := store.Get(context.Background(), "artifact-key")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer object.Body.Close()

	if client.getBucket != "protoradar" {
		t.Fatalf("bucket = %q", client.getBucket)
	}
	if object.Key != "artifact-key" {
		t.Fatalf("key = %q", object.Key)
	}
	if object.ContentType != artifactContentType {
		t.Fatalf("content type = %q", object.ContentType)
	}
	if object.SizeBytes != 8 {
		t.Fatalf("size = %d", object.SizeBytes)
	}
	body, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "artifact" {
		t.Fatalf("body = %q", string(body))
	}
}

func TestArtifactStoreDelete(t *testing.T) {
	client := &fakeClient{}
	store := newArtifactStoreWithClient("protoradar", client)

	if err := store.Delete(context.Background(), "artifact-key"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if client.deleteBucket != "protoradar" {
		t.Fatalf("bucket = %q", client.deleteBucket)
	}
	if client.deleteKey != "artifact-key" {
		t.Fatalf("key = %q", client.deleteKey)
	}
}

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   string
		wantHost   string
		wantSecure bool
		wantErr    bool
	}{
		{name: "host only", endpoint: "s3.example.com", wantHost: "s3.example.com", wantSecure: true},
		{name: "http", endpoint: "http://localhost:9000", wantHost: "localhost:9000", wantSecure: false},
		{name: "https", endpoint: "https://s3.example.com", wantHost: "s3.example.com", wantSecure: true},
		{name: "empty", endpoint: "", wantErr: true},
		{name: "unsupported scheme", endpoint: "ftp://s3.example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, gotSecure, err := parseEndpoint(tt.endpoint)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotHost != tt.wantHost {
				t.Fatalf("host = %q, want %q", gotHost, tt.wantHost)
			}
			if gotSecure != tt.wantSecure {
				t.Fatalf("secure = %v, want %v", gotSecure, tt.wantSecure)
			}
		})
	}
}

type fakeClient struct {
	putBucket      string
	putKey         string
	putSize        int64
	putContentType string

	getBucket string
	getKey    string
	getObject storedObject

	deleteBucket string
	deleteKey    string
}

func (client *fakeClient) put(ctx context.Context, bucket string, key string, body io.Reader, sizeBytes int64, contentType string) (int64, error) {
	client.putBucket = bucket
	client.putKey = key
	client.putSize = sizeBytes
	client.putContentType = contentType
	return sizeBytes, nil
}

func (client *fakeClient) get(ctx context.Context, bucket string, key string) (storedObject, error) {
	client.getBucket = bucket
	client.getKey = key
	return client.getObject, nil
}

func (client *fakeClient) delete(ctx context.Context, bucket string, key string) error {
	client.deleteBucket = bucket
	client.deleteKey = key
	return nil
}
