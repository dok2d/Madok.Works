package main

import (
	"context"
	"flag"
	"kanban/internal/db"
	"kanban/internal/handler"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "/data/kanban.db", "database: a SQLite file path, or a postgres://… / postgresql://… URL")
	dataDir := flag.String("data-dir", "", "directory for on-disk artifacts like backups (defaults to the SQLite file's directory, or /data for Postgres)")
	verbose := flag.Bool("verbose", false, "enable verbose logging")
	smtpAddr := flag.String("smtp-addr", "", "optional inbound SMTP listen address (e.g. :25); empty disables mail receiving")
	resetUser := flag.String("reset-user", "", "set this user's password and exit (creates an admin if the user doesn't exist) — for recovery on closed networks")
	resetPassword := flag.String("reset-password", "", "the new password for -reset-user")
	flag.Parse()

	// Resolve the data directory first (it may hold a persisted DB choice).
	dir := *dataDir
	if dir == "" {
		if strings.HasPrefix(*dbPath, "postgres://") || strings.HasPrefix(*dbPath, "postgresql://") {
			dir = "/data"
		} else {
			dir = filepath.Dir(*dbPath)
		}
	}

	// Effective DB: explicit postgres:// flag > a DSN persisted by first-run
	// setup (<data-dir>/db.json) > the -db value as a SQLite path.
	dsn := handler.ResolveDSN(*dbPath, dir)
	store, err := db.New(dsn)
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer store.Close()

	// CLI password recovery (no server start). Useful on closed networks where
	// the Telegram reset is unreachable — e.g. after restoring a backup, whose
	// users carry no password hash.
	if *resetUser != "" {
		if len(*resetPassword) < 6 {
			log.Fatalf("reset-password must be at least 6 characters")
		}
		if u, _ := store.GetUserByUsername(*resetUser); u != nil {
			if err := store.UpdateUserPassword(u.ID, *resetPassword); err != nil {
				log.Fatalf("reset password: %v", err)
			}
			log.Printf("password updated for user %q (id %d)", u.Username, u.ID)
		} else {
			id, err := store.CreateUser(*resetUser, *resetPassword, "admin")
			if err != nil {
				log.Fatalf("create user: %v", err)
			}
			log.Printf("created admin user %q (id %d)", *resetUser, id)
		}
		return
	}

	h := handler.New(store, dir)
	h.SetInstanceID(*addr)
	if *verbose {
		h.SetVerbose(true)
		log.Println("verbose logging enabled")
	}
	if *smtpAddr != "" {
		h.StartSMTPServer(*smtpAddr)
	}

	srv := &http.Server{
		Addr:    *addr,
		Handler: h,
		// ReadHeaderTimeout (not ReadTimeout) bounds slowloris on headers but lets
		// large request bodies (e.g. a backup import) upload without being cut off
		// by a whole-request read deadline. Handlers extend the write window where
		// needed (import, LLM) via http.ResponseController.
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Clean expired sessions periodically
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			store.CleanExpiredSessions()
		}
	}()

	go func() {
		log.Printf("kanban listening on %s", *addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
