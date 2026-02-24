package storage

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MediaStore interface {
	UploadImage(ctx context.Context, fileName string, contentType string, content []byte) (url string, key string, err error)
	DeleteObject(ctx context.Context, objectKey string) error
	ObjectKeyFromURL(rawURL string) string
}

type S3MediaStore struct {
	client        *minio.Client
	bucket        string
	publicBaseURL string
}

func NewS3MediaStore(endpoint, accessKey, secretKey, bucket string, useSSL bool, publicBaseURL string) (*S3MediaStore, error) {
	trimmedEndpoint := strings.TrimSpace(endpoint)
	if trimmedEndpoint == "" {
		return nil, fmt.Errorf("s3 endpoint is required")
	}
	trimmedEndpoint = strings.TrimPrefix(strings.TrimPrefix(trimmedEndpoint, "http://"), "https://")

	client, err := minio.New(trimmedEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create s3 client: %w", err)
	}

	if bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}

	exists, err := client.BucketExists(context.Background(), bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("create bucket: %w", err)
		}
	}

	resolvedPublicBaseURL := strings.TrimSpace(publicBaseURL)
	if resolvedPublicBaseURL == "" {
		scheme := "http"
		if useSSL {
			scheme = "https"
		}
		resolvedPublicBaseURL = fmt.Sprintf("%s://%s/%s", scheme, trimmedEndpoint, bucket)
	}

	return &S3MediaStore{
		client:        client,
		bucket:        bucket,
		publicBaseURL: strings.TrimRight(resolvedPublicBaseURL, "/"),
	}, nil
}

func (store *S3MediaStore) UploadImage(ctx context.Context, fileName string, contentType string, content []byte) (string, string, error) {
	if len(content) == 0 {
		return "", "", fmt.Errorf("empty file")
	}

	ext := strings.ToLower(path.Ext(fileName))
	if ext == "" {
		extensions, err := mime.ExtensionsByType(contentType)
		if err == nil && len(extensions) > 0 {
			ext = strings.ToLower(extensions[0])
		}
	}
	if ext == "" {
		ext = ".bin"
	}

	objectKey := fmt.Sprintf("images/%s%s", uuid.NewString(), ext)
	_, err := store.client.PutObject(ctx, store.bucket, objectKey, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", "", fmt.Errorf("upload object: %w", err)
	}

	return store.publicBaseURL + "/" + objectKey, objectKey, nil
}

func (store *S3MediaStore) DeleteObject(ctx context.Context, objectKey string) error {
	if objectKey == "" {
		return nil
	}
	err := store.client.RemoveObject(ctx, store.bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("delete object %s: %w", objectKey, err)
	}
	return nil
}

// ObjectKeyFromURL extracts the S3 object key from a full public URL.
// Returns empty string if the URL doesn't belong to this store.
func (store *S3MediaStore) ObjectKeyFromURL(rawURL string) string {
	prefix := store.publicBaseURL + "/"
	if strings.HasPrefix(rawURL, prefix) {
		return strings.TrimPrefix(rawURL, prefix)
	}
	return ""
}
