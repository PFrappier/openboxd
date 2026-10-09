package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"codeberg.org/pfrappier/openboxd/backend/internal/imports"
)

func main() {
	addr := envOr("ADDR", ":8080")
	dataDir := envOr("DATA_DIR", "data")

	importHandler := imports.NewHandler(filepath.Join(dataDir, "imports"))

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Post("/imports", importHandler.Upload)
	})

	slog.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
