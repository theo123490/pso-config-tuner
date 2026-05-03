package api

// RegisterRequest is the payload for POST /register.
type RegisterRequest struct {
	ID             string            `json:"id"`
	HealthcheckURL string            `json:"healthcheck_url"`
	ConfigKeys     []string          `json:"config_keys"`
	Defaults       map[string]float64 `json:"defaults,omitempty"`
}

// RegisterResponse is returned by POST /register.
type RegisterResponse struct {
	ParticleID string `json:"particle_id"`
	SwarmSize  int    `json:"swarm_size"`
}

// ConfigResponse is returned by GET /config/{particle_id}.
type ConfigResponse struct {
	Iteration         int                `json:"iteration"`
	ObservationWindow string             `json:"observation_window"`
	Config            map[string]float64 `json:"config"`
}

// ReportRequest is the payload for POST /report/{particle_id}.
type ReportRequest struct {
	Iteration int                    `json:"iteration"`
	Metrics   map[string]interface{} `json:"metrics"`
}

// ReportResponse is returned by POST /report/{particle_id}.
type ReportResponse struct {
	Status string `json:"status"`
}

// StatusResponse is returned by GET /status.
type StatusResponse struct {
	Iteration    int                `json:"iteration"`
	MaxIter      int                `json:"max_iter"`
	Converged    bool               `json:"converged"`
	GBestFitness float64            `json:"gbest_fitness"`
	GBestConfig  map[string]float64 `json:"gbest_config"`
}
