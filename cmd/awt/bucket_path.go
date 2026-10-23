package main

import (
	"errors"
	"fmt"
	"io"

	"awt/internal/config"
)

// bucketPathCmd prints the Litestream replica path for AWT_BUCKET_PREFIX
// (`awt bucket-path litestream`). It reads only that variable, so it works
// in any container that has the bucket settings, and fails on an invalid
// prefix so the entrypoint stops the container.
func bucketPathCmd(args []string, getenv func(string) string, out io.Writer) error {
	if len(args) != 1 || args[0] != "litestream" {
		return errors.New("usage: awt bucket-path litestream")
	}
	prefix, err := config.ParseBucketPrefix(getenv("AWT_BUCKET_PREFIX"))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, config.Config{BucketPrefix: prefix}.LitestreamReplicaPath())
	return err
}
