// Package api implements the Controller REST handlers.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/theodore-chandra/pso-config-tuner/internal/controller"
)

// Handler bundles all HTTP handler dependencies.
type Handler struct {
	ctrl *controller.Controller
}

// NewHandler creates a Handler with the given controller.
func NewHandler(ctrl *controller.Controller) *Handler {
	return &Handler{ctrl: ctrl}
}

// Register wires routes onto the given mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /register", h.handleRegister)
	mux.HandleFunc("GET /config/{particle_id}", h.handleConfig)
	mux.HandleFunc("POST /report/{particle_id}", h.handleReport)
	mux.HandleFunc("GET /status", h.handleStatus)
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}

	out, err := h.ctrl.Register(r.Context(), controller.RegisterInput{
		ID:             req.ID,
		HealthcheckURL: req.HealthcheckURL,
		ConfigKeys:     req.ConfigKeys,
		Defaults:       req.Defaults,
	})
	if err != nil {
		slog.Error("register failed", "particle_id", req.ID, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, RegisterResponse{
		ParticleID: out.ParticleID,
		SwarmSize:  out.SwarmSize,
	})
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("particle_id")
	out, err := h.ctrl.Config(r.Context(), pid)
	if err != nil {
		if isNotFound(err) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, ConfigResponse{
		Iteration:         out.Iteration,
		ObservationWindow: out.ObservationWindow,
		Config:            out.Config,
	})
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	pid := r.PathValue("particle_id")
	var req ReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	out, err := h.ctrl.SubmitReport(r.Context(), pid, controller.ReportInput{
		Iteration: req.Iteration,
		Metrics:   req.Metrics,
	})
	if err != nil {
		if isNotFound(err) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, ReportResponse{Status: out.Status})
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	out := h.ctrl.Status(r.Context())
	writeJSON(w, http.StatusOK, StatusResponse{
		Iteration:    out.Iteration,
		MaxIter:      out.MaxIter,
		Converged:    out.Converged,
		GBestFitness: out.GBestFitness,
		GBestConfig:  out.GBestConfig,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasPrefix(err.Error(), "unknown ")
}
