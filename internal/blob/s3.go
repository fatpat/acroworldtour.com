package blob

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Config configures the S3-compatible backend (OVHcloud Object Storage).
type S3Config struct {
	Endpoint  string // host[:port], no scheme; UseSSL selects TLS
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
	// Prefix is config.Config.BucketPrefix, already normalised: "" or
	// "<segments>/" (decision 0017).
	Prefix string
}

// S3 stores blobs in an S3-compatible bucket (decision 0011).
type S3 struct {
	client *minio.Client
	bucket string
	prefix string
}

// objectKey puts uploads under <prefix>files/, apart from the Litestream
// replica under <prefix>litestream, so one bucket can serve one environment
// or several (decision 0017).
func (s *S3) objectKey(key string) string { return s.prefix + "files/" + key }

// NewS3 builds the client; it does not contact the server.
func NewS3(cfg S3Config) (*S3, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, errors.New("s3: endpoint and bucket are required")
	}
	c, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}
	return &S3{client: c, bucket: cfg.Bucket, prefix: cfg.Prefix}, nil
}

// Put uploads r with an unknown size (streamed multipart).
func (s *S3) Put(ctx context.Context, key, contentType string, r io.Reader) error {
	_, err := s.client.PutObject(ctx, s.bucket, s.objectKey(key), r, -1, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("s3 put %s: %w", key, err)
	}
	return nil
}

// Get returns the object and its content type.
func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, s.objectKey(key), minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("s3 get %s: %w", key, err)
	}
	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("s3 get %s: %w", key, err)
	}
	return obj, info.ContentType, nil
}

// Delete removes the object; S3 treats a missing key as success.
func (s *S3) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, s.objectKey(key), minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3 delete %s: %w", key, err)
	}
	return nil
}

var (
	_ Store = (*FS)(nil)
	_ Store = (*S3)(nil)
)
