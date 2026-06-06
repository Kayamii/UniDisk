// Command unidisk is the UniDisk server: a single binary that serves the JSON
// API and (in production) the built web dashboard.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/unidisk/unidisk/internal/api"
	"github.com/unidisk/unidisk/internal/auth"
	"github.com/unidisk/unidisk/internal/config"
	"github.com/unidisk/unidisk/internal/crypto"
	"github.com/unidisk/unidisk/internal/pool"
	"github.com/unidisk/unidisk/internal/provider"
	"github.com/unidisk/unidisk/internal/provider/box"
	"github.com/unidisk/unidisk/internal/provider/dropbox"
	"github.com/unidisk/unidisk/internal/provider/googledrive"
	"github.com/unidisk/unidisk/internal/provider/onedrive"
	"github.com/unidisk/unidisk/internal/provider/pcloud"
	"github.com/unidisk/unidisk/internal/provider/s3"
	"github.com/unidisk/unidisk/internal/store"
)

func main() {
	cfg := config.Load()

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	// Ensure the built-in Admin and Viewer roles exist (idempotent).
	if err := st.SeedRoles(context.Background()); err != nil {
		log.Fatalf("seed roles: %v", err)
	}

	// Encryption for provider credentials at rest. Uses UNIDISK_ENCRYPTION_KEY
	// if set, else a key generated once and persisted next to the database.
	encKey, err := crypto.LoadOrCreateKey(cfg.EncryptionKey, filepath.Join(cfg.DataDir, "unidisk.key"))
	if err != nil {
		log.Fatalf("encryption key: %v", err)
	}
	cryptoBox, err := crypto.New(encKey)
	if err != nil {
		log.Fatalf("crypto: %v", err)
	}

	tokens, err := auth.NewTokenManager(cfg.JWTSecret)
	if err != nil {
		log.Fatalf("token manager: %v", err)
	}
	authSvc := auth.NewService(st, tokens)

	// Seed the bootstrap administrator from env on first boot (no-op if users
	// already exist or the admin env vars are unset).
	if err := authSvc.SeedAdmin(context.Background(), cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Fatalf("seed admin: %v", err)
	}

	registry := provider.NewRegistry(
		googledrive.New(cfg.GoogleClientID, cfg.GoogleClientSecret),
		dropbox.New(cfg.DropboxAppKey, cfg.DropboxAppSecret),
		onedrive.New(cfg.OneDriveClientID, cfg.OneDriveClientSecret),
		box.New(cfg.BoxClientID, cfg.BoxClientSecret),
		pcloud.New(cfg.PCloudClientID, cfg.PCloudClientSecret),
		s3.New(), // manual credential form; always available
	)
	poolSvc := pool.NewService(st, registry, cryptoBox)

	srv := api.NewServer(st, authSvc, tokens, registry, poolSvc, cryptoBox, cfg.PublicURL)
	handler := srv.Handler(spaHandler())

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		// No write timeout: uploads/downloads stream and may run long.
	}

	go func() {
		log.Printf("UniDisk listening on %s", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}

// spaHandler serves the built frontend from ./web/dist if present (the
// single-container production layout). In development the directory is absent
// and Vite serves the SPA separately, so this returns nil.
func spaHandler() http.Handler {
	dist := "web/dist"
	if v := os.Getenv("UNIDISK_WEB_DIR"); v != "" {
		dist = v
	}
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
		return nil
	}
	fileServer := http.FileServer(http.Dir(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve the requested asset if it exists; otherwise fall back to
		// index.html so client-side routing works on deep links.
		clean := filepath.Clean(r.URL.Path)
		if _, err := fs.Stat(os.DirFS(dist), filepath.ToSlash(clean[1:])); err == nil || clean == "/" {
			if clean == "/" {
				http.ServeFile(w, r, filepath.Join(dist, "index.html"))
				return
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	})
}
