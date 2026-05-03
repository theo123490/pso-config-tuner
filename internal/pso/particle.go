package pso

// DimType is the type of a search space dimension.
type DimType string

const (
	DimInt         DimType = "int"
	DimFloat       DimType = "float"
	DimCategorical DimType = "categorical"
)

// Dimension describes one axis of the search space.
type Dimension struct {
	Name     string
	Type     DimType
	Min      float64
	Max      float64
	Values   []string // non-nil for categorical
	Default  float64
}

// Particle is a candidate configuration vector.
type Particle struct {
	ID            string
	Position      []float64
	Velocity      []float64
	PBest         []float64
	PBestFitness  float64
}

// NewParticle initialises a particle at the given position with zero velocity.
func NewParticle(id string, dims int) *Particle {
	pos := make([]float64, dims)
	return &Particle{
		ID:           id,
		Position:     pos,
		Velocity:     make([]float64, dims),
		PBest:        append([]float64(nil), pos...),
		PBestFitness: -1e18,
	}
}
