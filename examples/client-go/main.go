// Example Go client (particle). Dummy implementation: registers, pulls config each
// iteration, sleeps for observation_window, then echoes the received config back as metrics.
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

type registerReq struct {
	ID             string             `json:"id"`
	HealthcheckURL string             `json:"healthcheck_url"`
	ConfigKeys     []string           `json:"config_keys"`
	Defaults       map[string]float64 `json:"defaults"`
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

var configKeys = []string{"x1", "x2"}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	id := envOr("PARTICLE_ID", "client-0")
	controllerURL := envOr("CONTROLLER_URL", "http://localhost:8080")

	defaults := map[string]float64{"x1": 2.5, "x2": 2.5}

	reg := registerReq{
		ID:             id,
		HealthcheckURL: "",
		ConfigKeys:     configKeys,
		Defaults:       defaults,
	}

	// retry registration until controller is ready
	for {
		if err := postJSON(controllerURL+"/register", reg); err != nil {
			slog.Warn("register failed, retrying", "particle_id", id, "err", err)
			time.Sleep(2 * time.Second)
			continue
		}
		slog.Info("registered", "particle_id", id)
		break
	}

	for {
		resp, err := http.Get(fmt.Sprintf("%s/config/%s", controllerURL, id))
		if err != nil {
			slog.Warn("config fetch failed", "particle_id", id, "err", err)
			time.Sleep(2 * time.Second)
			continue
		}
		var cfg configResp
		json.NewDecoder(resp.Body).Decode(&cfg)
		resp.Body.Close()

		slog.Info("got config", "particle_id", id, "iteration", cfg.Iteration, "config", cfg.Config)

		window, _ := time.ParseDuration(cfg.ObservationWindow)
		if window == 0 {
			window = 5 * time.Second
		}
		time.Sleep(window)

		// dummy: echo config values back as metrics
		metrics := make(map[string]interface{}, len(cfg.Config))
		for k, v := range cfg.Config {
			metrics[k] = v
		}

		report := reportReq{Iteration: cfg.Iteration, Metrics: metrics}
		if err := postJSON(fmt.Sprintf("%s/report/%s", controllerURL, id), report); err != nil {
			slog.Warn("report failed", "particle_id", id, "iteration", cfg.Iteration, "err", err)
		} else {
			slog.Info("reported metrics", "particle_id", id, "iteration", cfg.Iteration)
		}
	}
}

func postJSON(url string, body interface{}) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
