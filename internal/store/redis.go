// Package store manages swarm state persistence in Redis.
// Key schema: pso:{swarm_id}:particle:{particle_id}:{field}
//
//	pso:{swarm_id}:particles
//	pso:{swarm_id}:gbest
//	pso:{swarm_id}:gbest_fitness
//	pso:{swarm_id}:iteration
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// Store is the interface for swarm state persistence.
type Store interface {
	SaveParticle(ctx context.Context, swarmID, particleID string, p ParticleState) error
	LoadParticle(ctx context.Context, swarmID, particleID string) (ParticleState, error)
	SaveGBest(ctx context.Context, swarmID string, pos []float64, fitness float64) error
	LoadGBest(ctx context.Context, swarmID string) (pos []float64, fitness float64, err error)
	IncrIteration(ctx context.Context, swarmID string) (int64, error)
	LoadIteration(ctx context.Context, swarmID string) (int64, error)

	RegisterParticle(ctx context.Context, swarmID, particleID string, meta ParticleMeta) error
	LoadParticleMeta(ctx context.Context, swarmID, particleID string) (ParticleMeta, error)
	AddToRoster(ctx context.Context, swarmID, particleID string) error
	LoadRoster(ctx context.Context, swarmID string) ([]string, error)
}

// ParticleState is the persisted PSO state of a single particle.
type ParticleState struct {
	Position     []float64
	Velocity     []float64
	PBest        []float64
	PBestFitness float64
	State        string // active | inactive | evicted
	SkipCount    int
	FailureCount int
}

// ParticleMeta is the persisted registration metadata of a single particle.
type ParticleMeta struct {
	ClientID       string
	HealthcheckURL string
	ConfigKeys     []string
	Defaults       map[string]float64
}

// RedisStore implements Store using Redis.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a RedisStore and pings to verify connectivity.
func NewRedisStore(addr string) (*RedisStore, error) {
	c := redis.NewClient(&redis.Options{Addr: addr})
	if err := c.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping %s: %w", addr, err)
	}
	return &RedisStore{client: c}, nil
}

func (s *RedisStore) SaveParticle(ctx context.Context, swarmID, particleID string, p ParticleState) error {
	pfx := particleKey(swarmID, particleID)
	posJSON, err := marshalFloats(p.Position)
	if err != nil {
		return err
	}
	velJSON, err := marshalFloats(p.Velocity)
	if err != nil {
		return err
	}
	pbJSON, err := marshalFloats(p.PBest)
	if err != nil {
		return err
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, pfx+":pos", posJSON, 0)
	pipe.Set(ctx, pfx+":vel", velJSON, 0)
	pipe.Set(ctx, pfx+":pbest", pbJSON, 0)
	pipe.Set(ctx, pfx+":pbest_fitness", strconv.FormatFloat(p.PBestFitness, 'f', -1, 64), 0)
	pipe.Set(ctx, pfx+":state", p.State, 0)
	pipe.Set(ctx, pfx+":skip_count", strconv.Itoa(p.SkipCount), 0)
	pipe.Set(ctx, pfx+":failure_count", strconv.Itoa(p.FailureCount), 0)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) LoadParticle(ctx context.Context, swarmID, particleID string) (ParticleState, error) {
	pfx := particleKey(swarmID, particleID)
	pipe := s.client.Pipeline()
	posCmd := pipe.Get(ctx, pfx+":pos")
	velCmd := pipe.Get(ctx, pfx+":vel")
	pbCmd := pipe.Get(ctx, pfx+":pbest")
	pbfCmd := pipe.Get(ctx, pfx+":pbest_fitness")
	stCmd := pipe.Get(ctx, pfx+":state")
	skCmd := pipe.Get(ctx, pfx+":skip_count")
	fcCmd := pipe.Get(ctx, pfx+":failure_count")
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return ParticleState{}, err
	}

	var p ParticleState
	var err error
	if p.Position, err = unmarshalFloats(posCmd.Val()); err != nil {
		return p, fmt.Errorf("pos: %w", err)
	}
	if p.Velocity, err = unmarshalFloats(velCmd.Val()); err != nil {
		return p, fmt.Errorf("vel: %w", err)
	}
	if p.PBest, err = unmarshalFloats(pbCmd.Val()); err != nil {
		return p, fmt.Errorf("pbest: %w", err)
	}
	if pbfCmd.Val() != "" {
		if p.PBestFitness, err = strconv.ParseFloat(pbfCmd.Val(), 64); err != nil {
			return p, fmt.Errorf("pbest_fitness: %w", err)
		}
	}
	p.State = stCmd.Val()
	if skCmd.Val() != "" {
		if p.SkipCount, err = strconv.Atoi(skCmd.Val()); err != nil {
			return p, fmt.Errorf("skip_count: %w", err)
		}
	}
	if fcCmd.Val() != "" {
		if p.FailureCount, err = strconv.Atoi(fcCmd.Val()); err != nil {
			return p, fmt.Errorf("failure_count: %w", err)
		}
	}
	return p, nil
}

func (s *RedisStore) SaveGBest(ctx context.Context, swarmID string, pos []float64, fitness float64) error {
	pfx := swarmKey(swarmID)
	posJSON, err := marshalFloats(pos)
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, pfx+":gbest", posJSON, 0)
	pipe.Set(ctx, pfx+":gbest_fitness", strconv.FormatFloat(fitness, 'f', -1, 64), 0)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) LoadGBest(ctx context.Context, swarmID string) ([]float64, float64, error) {
	pfx := swarmKey(swarmID)
	pipe := s.client.Pipeline()
	posCmd := pipe.Get(ctx, pfx+":gbest")
	fitCmd := pipe.Get(ctx, pfx+":gbest_fitness")
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, 0, err
	}

	if posCmd.Val() == "" {
		return nil, 0, nil
	}
	pos, err := unmarshalFloats(posCmd.Val())
	if err != nil {
		return nil, 0, fmt.Errorf("gbest pos: %w", err)
	}
	fit, err := strconv.ParseFloat(fitCmd.Val(), 64)
	if err != nil {
		return nil, 0, fmt.Errorf("gbest fitness: %w", err)
	}
	return pos, fit, nil
}

func (s *RedisStore) IncrIteration(ctx context.Context, swarmID string) (int64, error) {
	return s.client.Incr(ctx, swarmKey(swarmID)+":iteration").Result()
}

func (s *RedisStore) LoadIteration(ctx context.Context, swarmID string) (int64, error) {
	v, err := s.client.Get(ctx, swarmKey(swarmID)+":iteration").Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

func (s *RedisStore) RegisterParticle(ctx context.Context, swarmID, particleID string, meta ParticleMeta) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, particleKey(swarmID, particleID)+":meta", b, 0).Err()
}

func (s *RedisStore) LoadParticleMeta(ctx context.Context, swarmID, particleID string) (ParticleMeta, error) {
	v, err := s.client.Get(ctx, particleKey(swarmID, particleID)+":meta").Result()
	if err != nil {
		return ParticleMeta{}, err
	}
	var meta ParticleMeta
	return meta, json.Unmarshal([]byte(v), &meta)
}

func (s *RedisStore) AddToRoster(ctx context.Context, swarmID, particleID string) error {
	return s.client.SAdd(ctx, swarmKey(swarmID)+":particles", particleID).Err()
}

func (s *RedisStore) LoadRoster(ctx context.Context, swarmID string) ([]string, error) {
	return s.client.SMembers(ctx, swarmKey(swarmID)+":particles").Result()
}

func swarmKey(swarmID string) string {
	return "pso:" + swarmID
}

func particleKey(swarmID, particleID string) string {
	return "pso:" + swarmID + ":particle:" + particleID
}

func marshalFloats(fs []float64) (string, error) {
	b, err := json.Marshal(fs)
	return string(b), err
}

func unmarshalFloats(s string) ([]float64, error) {
	if s == "" {
		return nil, nil
	}
	var fs []float64
	return fs, json.Unmarshal([]byte(s), &fs)
}
