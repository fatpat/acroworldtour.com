// Command awt is the single AcroWorldTour binary.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"awt/internal/auth"
	"awt/internal/cache"
	"awt/internal/config"
	"awt/internal/store"
	"awt/internal/web"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: awt serve|migrate|import|user")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		if err := serve(); err != nil {
			slog.Error("serve", "err", err)
			os.Exit(1)
		}
	case "migrate":
		if err := migrate(); err != nil {
			slog.Error("migrate", "err", err)
			os.Exit(1)
		}
	case "import":
		if err := importCmd(os.Args[2:], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "import:", err)
			os.Exit(1)
		}
	case "user":
		if err := userCmd(os.Args[2:], os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, "user:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		os.Exit(2)
	}
}

func serve() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	var handler slog.Handler = slog.NewTextHandler(os.Stderr, nil)
	if cfg.Env == "prod" {
		handler = slog.NewJSONHandler(os.Stderr, nil)
	}
	slog.SetDefault(slog.New(handler))

	db, err := openMigrated(cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	var purger cache.Purger = cache.NoopPurger{}
	if cfg.CloudflareZoneID != "" && cfg.CloudflareToken != "" {
		purger = cache.NewCloudflarePurger(cfg.CloudflareZoneID, cfg.CloudflareToken)
	}
	if (cfg.CloudflareZoneID == "") != (cfg.CloudflareToken == "") {
		slog.Warn("cloudflare purge disabled: set both AWT_CLOUDFLARE_ZONE_ID and AWT_CLOUDFLARE_TOKEN")
	}
	srv := web.NewServer(web.Deps{Cfg: cfg, Auth: auth.New(db, cfg), DB: db, Purger: purger})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", cfg.Addr, "env", cfg.Env)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func migrate() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	db, err := openMigrated(cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	slog.Info("migrated", "db", cfg.DBPath)
	return nil
}

// openMigrated creates the database directory, opens the database and
// applies pending migrations. serve runs it at startup so a fresh or restored
// volume always has an up-to-date schema.
func openMigrated(cfg config.Config) (*store.DB, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := store.Migrate(context.Background(), db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
