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
		Addr:         *addr,
		Handler:      h,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
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
