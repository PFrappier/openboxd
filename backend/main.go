package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"codeberg.org/pfrappier/openboxd/backend/internal/database"
	"codeberg.org/pfrappier/openboxd/backend/internal/imports"
	"codeberg.org/pfrappier/openboxd/backend/internal/library"
)

// Defaults suit regular API calls. Routes that need more time, like uploads,
// extend their own deadlines with http.ResponseController.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 2 * time.Minute

	// How long in-flight requests get to finish once a stop signal arrives.
	shutdownTimeout = 30 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := envOr("ADDR", ":8080")
	dataDir := envOr("DATA_DIR", "data")

	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return err
	}
	db, err := database.Open(ctx, filepath.Join(dataDir, "openboxd.db"))
	if err != nil {
		return err
	}
	defer db.Close()

	store := library.NewStore(db)
	importHandler := imports.NewHandler(filepath.Join(dataDir, "imports"), store)
	libraryHandler := library.NewHandler(store)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Post("/imports", importHandler.Upload)
		r.Get("/imports/{id}", importHandler.Summary)
		r.Get("/watched", libraryHandler.Watched)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}
	// A second signal now kills the process instead of waiting for shutdown.
	stop()

	slog.Info("shutting down, waiting for in-flight requests", "timeout", shutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	slog.Info("server stopped")
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
