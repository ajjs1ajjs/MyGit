package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ajjs1ajjs/MyGit/internal/api"
	"github.com/ajjs1ajjs/MyGit/internal/auth"
	"github.com/ajjs1ajjs/MyGit/internal/config"
	"github.com/ajjs1ajjs/MyGit/internal/git"
	"github.com/ajjs1ajjs/MyGit/internal/storage"
)

const Version = "3.2.0"

func main() {
	// Handle version flags before flag.Parse, which would otherwise reject
	// "--version" as an unknown flag and exit non-zero (breaking install.sh's
	// `mygit --version` sanity check).
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-version", "-v", "version":
			fmt.Println(Version)
			return
		}
	}
	port := flag.Int("port", 0, "listen port")
	flag.Parse()

	cfg := config.Default()
	if *port != 0 {
		cfg.Port = *port
	}
	if err := os.MkdirAll(cfg.RepoRoot, 0o755); err != nil {
		log.Fatalf("repo root: %v", err)
	}
	if _, err := cfg.EnsureJWTSecret(); err != nil {
		log.Fatalf("jwt secret: %v", err)
	}
	if _, err := cfg.EnsureInternalToken(); err != nil {
		log.Fatalf("internal token: %v", err)
	}

	db, abs, err := storage.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	store := storage.NewStore(db, abs)

	authn := auth.New(cfg.JWTSecret, cfg.JWTExpireMin, cfg.RefreshExpireDays)
	gitBackend := git.New(cfg.GitBinary, cfg.RepoRoot)

	app := &api.App{Cfg: cfg, Store: store, Auth: authn, Git: gitBackend, Start: time.Now()}
	app.StartBackupScheduler()

	// dual-stack bind (a lesson from the other ports).
	var addr string
	if cfg.Host == "" || cfg.Host == "0.0.0.0" {
		addr = fmt.Sprintf(":%d", cfg.Port)
	} else {
		addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	}
	srv := &http.Server{Addr: addr, Handler: app.Handler()}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		log.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	scheme := "http"
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		scheme = "https"
	}
	log.Printf("MyGit %s listening on %s://%s (repos: %s)", Version, scheme, addr, cfg.RepoRoot)
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		err = srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
	} else {
		// No TLS configured: either run behind a reverse proxy that terminates
		// TLS, or bind to loopback only for a trusted network.
		log.Println("TLS disabled (set MYGIT_TLS_CERT and MYGIT_TLS_KEY, or terminate TLS at a reverse proxy)")
		err = srv.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}
