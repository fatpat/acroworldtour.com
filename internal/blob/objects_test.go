package blob_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"

	"awt/internal/blob"
)

type memObject struct {
	data        []byte
	contentType string
}

// memBucket is an in-memory blob.Bucket. With server set, it copies
// server-side from another memBucket of the same server.
type memBucket struct {
	mu      sync.Mutex
	server  string
	objects map[string]memObject
	copies  int // server-side copies made into this bucket
	writes  int // streamed writes into this bucket
	failGet string
}

func newMem(server string, objs map[string]memObject) *memBucket {
	if objs == nil {
		objs = map[string]memObject{}
	}
	return &memBucket{server: server, objects: objs}
}

func (m *memBucket) List(_ context.Context, prefix string) ([]blob.Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []blob.Object
	for _, k := range slices.Sorted(maps.Keys(m.objects)) {
		if strings.HasPrefix(k, prefix) {
			out = append(out, blob.Object{Key: k, Size: int64(len(m.objects[k].data))})
		}
	}
	return out, nil
}

func (m *memBucket) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if key == m.failGet {
		return nil, "", errors.New("boom")
	}
	o, ok := m.objects[key]
	if !ok {
		return nil, "", blob.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(o.data)), o.contentType, nil
}

func (m *memBucket) Write(_ context.Context, key, contentType string, r io.Reader, size int64) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if int64(len(b)) != size {
		return errors.New("size mismatch")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = memObject{b, contentType}
	m.writes++
	return nil
}

func (m *memBucket) Remove(_ context.Context, keys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.objects, k)
	}
	return nil
}

func (m *memBucket) CanCopyFrom(src blob.Bucket) bool {
	s, ok := src.(*memBucket)
	return ok && m.server != "" && s.server == m.server
}

func (m *memBucket) CopyFrom(_ context.Context, src blob.Bucket, srcKey, dstKey string) error {
	s := src.(*memBucket)
	s.mu.Lock()
	o, ok := s.objects[srcKey]
	s.mu.Unlock()
	if !ok {
		return blob.ErrNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[dstKey] = o
	m.copies++
	return nil
}

func keys(m *memBucket) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Sorted(maps.Keys(m.objects))
}

func TestFilesPrefix(t *testing.T) {
	for in, want := range map[string]string{"": "files/", "prod/": "prod/files/", "a/b/": "a/b/files/"} {
		if got := blob.FilesPrefix(in); got != want {
			t.Errorf("FilesPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeletePrefix(t *testing.T) {
	ctx := context.Background()
	b := newMem("", map[string]memObject{
		"training/files/a":                {data: []byte("a")},
		"training/files/b":                {data: []byte("b")},
		"training/files-other/c":          {data: []byte("c")},
		"training/litestream/generations": {data: []byte("g")},
		"prod/files/a":                    {data: []byte("a")},
	})
	n, err := blob.DeletePrefix(ctx, b, "training/files/")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("deleted %d, want 2", n)
	}
	want := []string{"prod/files/a", "training/files-other/c", "training/litestream/generations"}
	if got := keys(b); !slices.Equal(got, want) {
		t.Errorf("left %v, want %v", got, want)
	}
	// Idempotent: nothing left to delete is not an error.
	if n, err := blob.DeletePrefix(ctx, b, "training/files/"); err != nil || n != 0 {
		t.Errorf("second delete: %d, %v", n, err)
	}
}

// A prefix that does not end in "/" could match a sibling ("files" would
// also match "files-old/..."); an empty one is the whole bucket.
func TestDeletePrefixRefusesUnboundedPrefix(t *testing.T) {
	b := newMem("", map[string]memObject{"x": {data: []byte("x")}})
	for _, p := range []string{"", "files", "/"} {
		if _, err := blob.DeletePrefix(context.Background(), b, p); err == nil {
			t.Errorf("prefix %q accepted", p)
		}
	}
	if len(keys(b)) != 1 {
		t.Error("objects deleted")
	}
}

func TestCopyPrefixStreams(t *testing.T) {
	ctx := context.Background()
	src := newMem("one", map[string]memObject{
		"prod/files/a":      {[]byte("aaa"), "image/png"},
		"prod/files/b":      {[]byte("bb"), "application/pdf"},
		"prod/litestream/x": {[]byte("x"), ""},
	})
	dst := newMem("two", map[string]memObject{"files/old": {[]byte("o"), "text/plain"}})
	res, err := blob.CopyPrefix(ctx, src, "prod/files/", dst, "files/")
	if err != nil {
		t.Fatal(err)
	}
	if res.Objects != 2 || res.Bytes != 5 || res.ServerSide {
		t.Errorf("result %+v", res)
	}
	if dst.writes != 2 || dst.copies != 0 {
		t.Errorf("writes %d copies %d", dst.writes, dst.copies)
	}
	if got, want := keys(dst), []string{"files/a", "files/b", "files/old"}; !slices.Equal(got, want) {
		t.Errorf("dst keys %v, want %v", got, want)
	}
	if o := dst.objects["files/b"]; string(o.data) != "bb" || o.contentType != "application/pdf" {
		t.Errorf("files/b = %q %q", o.data, o.contentType)
	}
	if len(keys(src)) != 3 {
		t.Error("source changed")
	}
}

func TestCopyPrefixServerSide(t *testing.T) {
	ctx := context.Background()
	src := newMem("same", map[string]memObject{"prod/files/a": {[]byte("aaa"), "image/png"}})
	dst := newMem("same", nil)
	res, err := blob.CopyPrefix(ctx, src, "prod/files/", dst, "training/files/")
	if err != nil {
		t.Fatal(err)
	}
	if !res.ServerSide || res.Objects != 1 || dst.copies != 1 || dst.writes != 0 {
		t.Errorf("result %+v copies %d writes %d", res, dst.copies, dst.writes)
	}
	if o := dst.objects["training/files/a"]; string(o.data) != "aaa" || o.contentType != "image/png" {
		t.Errorf("copy = %q %q", o.data, o.contentType)
	}
}

func TestCopyPrefixFails(t *testing.T) {
	ctx := context.Background()
	src := newMem("", map[string]memObject{"p/files/a": {[]byte("a"), "x/y"}})
	src.failGet = "p/files/a"
	if _, err := blob.CopyPrefix(ctx, src, "p/files/", newMem("", nil), "q/files/"); err == nil || !strings.Contains(err.Error(), "p/files/a") {
		t.Errorf("err = %v", err)
	}
	for _, p := range [][2]string{{"", "q/files/"}, {"p/files/", ""}, {"p/files", "q/files/"}} {
		if _, err := blob.CopyPrefix(ctx, src, p[0], newMem("", nil), p[1]); err == nil {
			t.Errorf("prefixes %q accepted", p)
		}
	}
}
