// Package api implements the Controller REST handlers.
package api

import "net/http"

// Handler bundles all HTTP handler dependencies.
type Handler struct {
	// TODO(M3): inject swarm state, store, config
}

// Register wires routes onto the given mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /register", h.handleRegister)
	mux.HandleFunc("GET /config/{particle_id}", h.handleConfig)
	mux.HandleFunc("POST /report/{particle_id}", h.handleReport)
	mux.HandleFunc("GET /status", h.handleStatus)
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
