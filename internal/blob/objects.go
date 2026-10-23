package blob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// FilesPrefix is where an environment's uploads live in its bucket:
// <bucket prefix>files/ (decision 0017). bucketPrefix is
// config.Config.BucketPrefix, already normalised.
func FilesPrefix(bucketPrefix string) string { return bucketPrefix + "files/" }

// Object is one entry of a bucket listing.
type Object struct {
	Key  string
	Size int64
}

// Bucket is a raw view of one bucket, addressed by full object keys. It
// serves whole-prefix operations (the training refresh, docs/ops.md), not
// the app's uploads, which go through Store.
type Bucket interface {
	// List returns every object whose key starts with prefix.
	List(ctx context.Context, prefix string) ([]Object, error)
	// Open returns the content and its content type.
	Open(ctx context.Context, key string) (io.ReadCloser, string, error)
	Write(ctx context.Context, key, contentType string, r io.Reader, size int64) error
	// Remove deletes the keys; a missing key is not an error.
	Remove(ctx context.Context, keys []string) error
}

// Copier is a Bucket that can copy objects from another bucket without
// downloading them (same S3 server and credentials).
type Copier interface {
	CanCopyFrom(src Bucket) bool
	// CopyFrom copies srcKey of src to dstKey, keeping its content type.
	CopyFrom(ctx context.Context, src Bucket, srcKey, dstKey string) error
}

// checkPrefix refuses a prefix that is empty (the whole bucket) or does not
// end in "/" ("files" would also match "files-old/...").
func checkPrefix(p string) error {
	if p == "" || p == "/" || !strings.HasSuffix(p, "/") {
		return fmt.Errorf("prefix %q: must be non-empty and end with /", p)
	}
	return nil
}

// removeBatch is the S3 DeleteObjects limit.
const removeBatch = 1000

// DeletePrefix deletes every object under prefix and returns how many.
func DeletePrefix(ctx context.Context, b Bucket, prefix string) (int, error) {
	if err := checkPrefix(prefix); err != nil {
		return 0, err
	}
	objs, err := b.List(ctx, prefix)
	if err != nil {
		return 0, fmt.Errorf("list %s: %w", prefix, err)
	}
	keys := make([]string, 0, len(objs))
	for _, o := range objs {
		keys = append(keys, o.Key)
	}
	for start := 0; start < len(keys); start += removeBatch {
		end := min(start+removeBatch, len(keys))
		if err := b.Remove(ctx, keys[start:end]); err != nil {
			return start, fmt.Errorf("delete under %s: %w", prefix, err)
		}
	}
	return len(keys), nil
}

// CopyResult reports a CopyPrefix.
type CopyResult struct {
	Objects    int
	Bytes      int64
	ServerSide bool
}

// CopyPrefix copies every object under srcPrefix of src to the same key
// relative to dstPrefix in dst, keeping content types. It copies
// server-side when dst can (Copier), otherwise it streams each object.
// Objects already in dst are overwritten; others there are left alone.
func CopyPrefix(ctx context.Context, src Bucket, srcPrefix string, dst Bucket, dstPrefix string) (CopyResult, error) {
	var res CopyResult
	if err := errors.Join(checkPrefix(srcPrefix), checkPrefix(dstPrefix)); err != nil {
		return res, err
	}
	objs, err := src.List(ctx, srcPrefix)
	if err != nil {
		return res, fmt.Errorf("list %s: %w", srcPrefix, err)
	}
	copier, ok := dst.(Copier)
	res.ServerSide = ok && copier.CanCopyFrom(src)
	for _, o := range objs {
		dstKey := dstPrefix + strings.TrimPrefix(o.Key, srcPrefix)
		if res.ServerSide {
			err = copier.CopyFrom(ctx, src, o.Key, dstKey)
		} else {
			err = stream(ctx, src, o, dst, dstKey)
		}
		if err != nil {
			return res, fmt.Errorf("copy %s to %s: %w", o.Key, dstKey, err)
		}
		res.Objects++
		res.Bytes += o.Size
	}
	return res, nil
}

func stream(ctx context.Context, src Bucket, o Object, dst Bucket, dstKey string) error {
	rc, ct, err := src.Open(ctx, o.Key)
	if err != nil {
		return err
	}
	defer rc.Close()
	return dst.Write(ctx, dstKey, ct, rc, o.Size)
}
