package pso

// Swarm holds the global best across all particles.
type Swarm struct {
	Dims          []Dimension
	GBest         []float64
	GBestFitness  float64
	Iteration     int
	// PSO hyperparameters
	Inertia   float64
	Cognitive float64
	Social    float64
}

// NewSwarm creates a swarm with Clerc-Kennedy defaults.
func NewSwarm(dims []Dimension) *Swarm {
	return &Swarm{
		Dims:         dims,
		GBest:        make([]float64, len(dims)),
		GBestFitness: -1e18,
		Inertia:      0.729,
		Cognitive:    1.49445,
		Social:       1.49445,
	}
}

// UpdateGBest updates the global best if the particle's pBest is better.
func (s *Swarm) UpdateGBest(p *Particle) bool {
	if p.PBestFitness > s.GBestFitness {
		s.GBestFitness = p.PBestFitness
		s.GBest = append([]float64(nil), p.PBest...)
		return true
	}
	return false
}
