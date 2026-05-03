package pso

import "math/rand"

// Update applies the standard PSO velocity and position update for one particle.
// Positions are clamped to dimension bounds after update.
func (s *Swarm) Update(p *Particle, rng *rand.Rand) {
	for i, dim := range s.Dims {
		r1 := rng.Float64()
		r2 := rng.Float64()

		p.Velocity[i] = s.Inertia*p.Velocity[i] +
			s.Cognitive*r1*(p.PBest[i]-p.Position[i]) +
			s.Social*r2*(s.GBest[i]-p.Position[i])

		p.Position[i] = clamp(p.Position[i]+p.Velocity[i], dim.Min, dim.Max)

		if dim.Type == DimInt {
			p.Position[i] = float64(int(p.Position[i] + 0.5))
		}
	}
}

// UpdatePBest updates a particle's personal best if the given fitness is higher.
func UpdatePBest(p *Particle, fitness float64) bool {
	if fitness > p.PBestFitness {
		p.PBestFitness = fitness
		p.PBest = append([]float64(nil), p.Position...)
		return true
	}
	return false
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
