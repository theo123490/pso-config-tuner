// Example Go client (particle). Registers with the Controller, then runs the
// iteration loop: pull config → apply → observe → report.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const controllerURL = "http://localhost:8080"

type registerReq struct {
	ID             string   `json:"id"`
	HealthcheckURL string   `json:"healthcheck_url"`
	ConfigKeys     []string `json:"config_keys"`
}

type configResp struct {
	Iteration         int                `json:"iteration"`
	ObservationWindow string             `json:"observation_window"`
	Config            map[string]float64 `json:"config"`
}

type reportReq struct {
	Iteration int                    `json:"iteration"`
	Metrics   map[string]interface{} `json:"metrics"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	id := envOr("PARTICLE_ID", "client-0")

	// Register
	reg := registerReq{
		ID:             id,
		HealthcheckURL: fmt.Sprintf("http://%s:9090/health", id),
		ConfigKeys:     []string{"worker_pool_size", "queue_depth", "timeout_ms"},
	}
	post(controllerURL+"/register", reg)
	slog.Info("registered", "particle_id", id)

	for {
		// Pull config
		resp, err := http.Get(fmt.Sprintf("%s/config/%s", controllerURL, id))
		if err != nil {
			slog.Error("config fetch failed", "err", err)
			time.Sleep(5 * time.Second)
			continue
		}
		var cfg configResp
		json.NewDecoder(resp.Body).Decode(&cfg)
		resp.Body.Close()

		slog.Info("got config", "iteration", cfg.Iteration, "config", cfg.Config)

		// Apply config and observe (placeholder — replace with real workload)
		window, _ := time.ParseDuration(cfg.ObservationWindow)
		if window == 0 {
			window = 30 * time.Second
		}
		time.Sleep(window)

		// Report metrics (placeholder values)
		report := reportReq{
			Iteration: cfg.Iteration,
			Metrics: map[string]interface{}{
				"p99_latency_ms": 120.0,
				"throughput_rps": 5000,
				"error_rate":     0.001,
			},
		}
		post(fmt.Sprintf("%s/report/%s", controllerURL, id), report)
		slog.Info("reported metrics", "iteration", cfg.Iteration)
	}
}

func post(url string, body interface{}) {
	b, _ := json.Marshal(body)
	http.Post(url, "application/json", bytes.NewReader(b)) //nolint:errcheck
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
