package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"awt/internal/blob"
	"awt/internal/config"
	"awt/internal/importer"
	"awt/internal/store"
)

const importUsage = "usage: awt import --from <export.tar.gz|dir> [--db path] [--blob-dir dir] [--catalogs dir] [--replace]"

// importCmd loads the production MongoDB export into the database.
func importCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	from := fs.String("from", "", "export .tar.gz or directory of <collection>.json files")
	catalogs := fs.String("catalogs", "testdata/catalog", "directory with the 2022-2025 trick catalogs (<year>.json)")
	dbFlag := fs.String("db", "", "database file (default: AWT_DB_PATH)")
	blobDir := fs.String("blob-dir", "", "directory for the file blobs (default: AWT_BLOB_DIR)")
	replace := fs.Bool("replace", false, "delete the imported tables' rows first instead of refusing a non-empty database")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *from == "" {
		return errors.New(importUsage)
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if *dbFlag != "" {
		cfg.DBPath = *dbFlag
	}
	if *blobDir != "" {
		cfg.BlobDir = *blobDir
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return fmt.Errorf("create db dir: %w", err)
	}
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}
	blobs, err := newBlobStore(cfg)
	if err != nil {
		return err
	}
	opts := []importer.Option{importer.WithCatalogDir(*catalogs)}
	if *replace {
		opts = append(opts, importer.WithReplace())
	}
	rep, err := importer.Run(ctx, db, blobs, *from, opts...)
	if err != nil {
		return err
	}
	printReport(out, rep)
	return nil
}

func newBlobStore(cfg config.Config) (blob.Store, error) {
	if cfg.BlobBackend == "s3" {
		return blob.NewS3(blob.S3Config{
			Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, UseSSL: true,
		})
	}
	return blob.NewFS(cfg.BlobDir)
}

func printReport(w io.Writer, rep importer.Report) {
	keys := make([]string, 0, len(rep.Counts))
	for k := range rep.Counts {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "%-24s %d\n", k, rep.Counts[k])
	}
	for _, msg := range rep.Warnings {
		fmt.Fprintf(w, "warning: %s\n", msg)
	}
}
