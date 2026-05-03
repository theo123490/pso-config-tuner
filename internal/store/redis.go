// Package store manages swarm state persistence in Redis.
// Key schema: pso:{swarm_id}:particle:{particle_id}:{field}
//             pso:{swarm_id}:gbest
//             pso:{swarm_id}:gbest_fitness
//             pso:{swarm_id}:iteration
package store

import "context"

// Store is the interface for swarm state persistence.
type Store interface {
	SaveParticle(ctx context.Context, swarmID, particleID string, p ParticleState) error
	LoadParticle(ctx context.Context, swarmID, particleID string) (ParticleState, error)
	SaveGBest(ctx context.Context, swarmID string, pos []float64, fitness float64) error
	LoadGBest(ctx context.Context, swarmID string) (pos []float64, fitness float64, err error)
	IncrIteration(ctx context.Context, swarmID string) (int64, error)
	LoadIteration(ctx context.Context, swarmID string) (int64, error)
}

// ParticleState is the persisted state of a single particle.
type ParticleState struct {
	Position     []float64
	Velocity     []float64
	PBest        []float64
	PBestFitness float64
	State        string // active | inactive | evicted
	SkipCount    int
	FailureCount int
}
