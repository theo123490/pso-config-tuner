package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/theodore-chandra/pso-config-tuner/internal/api"
	"github.com/theodore-chandra/pso-config-tuner/internal/config"
	"github.com/theodore-chandra/pso-config-tuner/internal/controller"
	"github.com/theodore-chandra/pso-config-tuner/internal/metrics"
	"github.com/theodore-chandra/pso-config-tuner/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// config precedence: env > yaml > compiled defaults
	cfgPath := envOr("CONFIG_PATH", "configs/swarm.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("load config failed", "path", cfgPath, "err", err)
		os.Exit(1)
	}

	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	st, err := store.NewRedisStore(redisAddr)
	if err != nil {
		slog.Error("redis connect failed", "addr", redisAddr, "err", err)
		os.Exit(1)
	}

	reg := prometheus.NewRegistry()
	m := metrics.New(reg)

	ctrl, err := controller.New(cfg, st, m)
	if err != nil {
		slog.Error("controller init failed", "err", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	api.NewHandler(ctrl).Register(mux)
	mux.Handle("GET /metrics", metrics.Handler(reg))

	addr := envOr("CONTROLLER_ADDR", ":8080")
	srv := &http.Server{Addr: addr, Handler: mux}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go ctrl.Run(ctx)

	go func() {
		slog.Info("controller starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("signal received, shutting down", "signal", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("http shutdown error", "err", err)
	}

	ctrl.Shutdown()
	cancel()

	select {
	case <-ctrl.Done():
		slog.Info("controller shut down cleanly")
	case <-time.After(30 * time.Second):
		slog.Warn("controller shutdown timed out")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
