package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"awt/internal/config"
	"awt/internal/importer"
	"awt/internal/store"
)

const importWWWUsage = "usage: awt import-www --from <legacy www dir> [--db path] [--blob-dir dir]\n" +
	"re-running overwrites news and events with the same slug but never restores ones deleted in the admin;\n" +
	"undated news take the date of the git commit that added them, else stay drafts"

// importWWWCmd imports the legacy static site (news, events, sporting code
// PDFs, redirects) into the content tables. It is idempotent.
func importWWWCmd(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("import-www", flag.ContinueOnError)
	from := fs.String("from", "", "legacy www directory (legacy/legacy/www)")
	dbFlag := fs.String("db", "", "database file (default: AWT_DB_PATH)")
	blobDir := fs.String("blob-dir", "", "directory for the file blobs (default: AWT_BLOB_DIR)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *from == "" {
		return errors.New(importWWWUsage)
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
	rep, err := importer.WWW(ctx, db, blobs, *from)
	if err != nil {
		return err
	}
	printReport(out, rep)
	return nil
}
