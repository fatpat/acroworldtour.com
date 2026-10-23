package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

// S3Bucket is the raw Bucket view of an S3 bucket (Prefix is ignored: keys
// are full object keys).
type S3Bucket struct {
	client *minio.Client
	bucket string
	// server identifies endpoint and credentials, for CanCopyFrom (a
	// hash: the struct never holds the secret in clear twice).
	server string
}

// NewS3Bucket builds the client; it does not contact the server.
func NewS3Bucket(cfg S3Config) (*S3Bucket, error) {
	c, err := newClient(cfg)
	if err != nil {
		return nil, err
	}
	u := c.EndpointURL()
	id := sha256.Sum256([]byte(u.Scheme + "://" + u.Host + "\x00" + cfg.AccessKey + "\x00" + cfg.SecretKey))
	return &S3Bucket{client: c, bucket: cfg.Bucket, server: hex.EncodeToString(id[:])}, nil
}

// List implements Bucket.
func (b *S3Bucket) List(ctx context.Context, prefix string) ([]Object, error) {
	var out []Object
	for o := range b.client.ListObjects(ctx, b.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if o.Err != nil {
			return nil, fmt.Errorf("s3 list %s/%s: %w", b.bucket, prefix, o.Err)
		}
		out = append(out, Object{Key: o.Key, Size: o.Size})
	}
	return out, nil
}

// Open implements Bucket.
func (b *S3Bucket) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	obj, err := b.client.GetObject(ctx, b.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("s3 get %s/%s: %w", b.bucket, key, err)
	}
	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("s3 get %s/%s: %w", b.bucket, key, err)
	}
	return obj, info.ContentType, nil
}

// Write implements Bucket.
func (b *S3Bucket) Write(ctx context.Context, key, contentType string, r io.Reader, size int64) error {
	_, err := b.client.PutObject(ctx, b.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("s3 put %s/%s: %w", b.bucket, key, err)
	}
	return nil
}

// Remove implements Bucket with batched DeleteObjects requests.
func (b *S3Bucket) Remove(ctx context.Context, keys []string) error {
	ch := make(chan minio.ObjectInfo, len(keys))
	for _, k := range keys {
		ch <- minio.ObjectInfo{Key: k}
	}
	close(ch)
	for e := range b.client.RemoveObjects(ctx, b.bucket, ch, minio.RemoveObjectsOptions{}) {
		if e.Err != nil {
			return fmt.Errorf("s3 delete %s/%s: %w", b.bucket, e.ObjectName, e.Err)
		}
	}
	return nil
}

// CanCopyFrom implements Copier: a server-side copy needs the same server
// and the same credentials (they must read the source and write here).
func (b *S3Bucket) CanCopyFrom(src Bucket) bool {
	s, ok := src.(*S3Bucket)
	return ok && s.server == b.server
}

// CopyFrom implements Copier. The object's metadata, content type
// included, is copied with it.
func (b *S3Bucket) CopyFrom(ctx context.Context, src Bucket, srcKey, dstKey string) error {
	s := src.(*S3Bucket)
	_, err := b.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: b.bucket, Object: dstKey},
		minio.CopySrcOptions{Bucket: s.bucket, Object: srcKey})
	if err != nil {
		return fmt.Errorf("s3 copy %s/%s to %s/%s: %w", s.bucket, srcKey, b.bucket, dstKey, err)
	}
	return nil
}

var (
	_ Bucket = (*S3Bucket)(nil)
	_ Copier = (*S3Bucket)(nil)
)
