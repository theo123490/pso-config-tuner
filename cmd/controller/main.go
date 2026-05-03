package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/theodore-chandra/pso-config-tuner/internal/api"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	mux := http.NewServeMux()

	h := &api.Handler{}
	h.Register(mux)

	addr := envOr("CONTROLLER_ADDR", ":8080")
	slog.Info("controller starting", "addr", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
