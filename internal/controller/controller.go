// Package controller orchestrates the PSO swarm: iteration loop, barrier, FC calls, health-check.
package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/theodore-chandra/pso-config-tuner/internal/config"
	"github.com/theodore-chandra/pso-config-tuner/internal/metrics"
	"github.com/theodore-chandra/pso-config-tuner/internal/pso"
	"github.com/theodore-chandra/pso-config-tuner/internal/store"
)

// RegisterInput is the input to Register.
type RegisterInput struct {
	ID             string
	HealthcheckURL string
	ConfigKeys     []string
	Defaults       map[string]float64
}

// RegisterOutput is the output from Register.
type RegisterOutput struct {
	ParticleID string
	SwarmSize  int
}

// ConfigOutput is the output from Config.
type ConfigOutput struct {
	Iteration         int
	ObservationWindow string
	Config            map[string]float64
}

// ReportInput is the input to SubmitReport.
type ReportInput struct {
	Iteration int
	Metrics   map[string]interface{}
}

// ReportOutput is the output from SubmitReport.
type ReportOutput struct {
	Status string
}

// StatusOutput is the output from Status.
type StatusOutput struct {
	Iteration    int
	MaxIter      int
	Converged    bool
	GBestFitness float64
	GBestConfig  map[string]float64
}

// Controller orchestrates the PSO swarm.
// All fields below the blank line are guarded by mu (RWMutex).
type Controller struct {
	cfg    *config.Config
	swarm  *pso.Swarm
	st     store.Store
	m      *metrics.Metrics
	rng    *rand.Rand
	fcHTTP *http.Client

	mu            sync.RWMutex
	particles     map[string]*pso.Particle     // guarded by mu
	particleMeta  map[string]store.ParticleMeta // guarded by mu
	particleState map[string]string             // active|inactive|evicted; guarded by mu
	skipCounts    map[string]int                // guarded by mu
	failureCounts map[string]int                // guarded by mu
	currentIter   int // guarded by mu

	barrier        *iterBarrier           // guarded by mu (pointer swap each iteration)
	pendingReports map[string]reportEntry // guarded by mu

	stopCh chan struct{}
	doneCh chan struct{}
}

type reportEntry struct {
	metrics map[string]interface{}
}

// New creates a Controller, loading existing swarm state from Redis if present.
func New(cfg *config.Config, st store.Store, m *metrics.Metrics) (*Controller, error) {
	swarm := pso.NewSwarm(cfg.Space)
	swarm.Inertia = cfg.PSO.Inertia
	swarm.Cognitive = cfg.PSO.Cognitive
	swarm.Social = cfg.PSO.Social

	c := &Controller{
		cfg:            cfg,
		swarm:          swarm,
		st:             st,
		m:              m,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
		fcHTTP:         &http.Client{Timeout: cfg.Swarm.ObservationWindow / 2},
		particles:      make(map[string]*pso.Particle),
		particleMeta:   make(map[string]store.ParticleMeta),
		particleState:  make(map[string]string),
		skipCounts:     make(map[string]int),
		failureCounts:  make(map[string]int),
		pendingReports: make(map[string]reportEntry),
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}

	if err := c.loadFromRedis(context.Background()); err != nil {
		return nil, fmt.Errorf("load swarm state: %w", err)
	}
	return c, nil
}

// loadFromRedis restores in-memory state from Redis on startup.
func (c *Controller) loadFromRedis(ctx context.Context) error {
	swarmID := c.cfg.Swarm.ClusterID

	iter, err := c.st.LoadIteration(ctx, swarmID)
	if err != nil {
		return err
	}
	c.currentIter = int(iter)

	gbestPos, gbestFit, err := c.st.LoadGBest(ctx, swarmID)
	if err != nil {
		return err
	}
	if gbestPos != nil {
		c.swarm.GBest = gbestPos
		c.swarm.GBestFitness = gbestFit
	}

	roster, err := c.st.LoadRoster(ctx, swarmID)
	if err != nil {
		return err
	}

	for _, pid := range roster {
		meta, err := c.st.LoadParticleMeta(ctx, swarmID, pid)
		if err != nil {
			slog.Warn("load particle meta failed", "swarm_id", swarmID, "particle_id", pid, "err", err)
			continue
		}
		ps, err := c.st.LoadParticle(ctx, swarmID, pid)
		if err != nil {
			slog.Warn("load particle state failed", "swarm_id", swarmID, "particle_id", pid, "err", err)
			continue
		}

		p := pso.NewParticle(pid, len(c.cfg.Space))
		if len(ps.Position) == len(c.cfg.Space) {
			p.Position = ps.Position
			p.Velocity = ps.Velocity
			p.PBest = ps.PBest
			p.PBestFitness = ps.PBestFitness
		}

		state := ps.State
		if state == "" {
			state = "active"
		}

		c.particles[pid] = p
		c.particleMeta[pid] = meta
		c.particleState[pid] = state
		c.skipCounts[pid] = ps.SkipCount
		c.failureCounts[pid] = ps.FailureCount
	}

	slog.Info("swarm state loaded", "swarm_id", swarmID, "iteration", c.currentIter, "particles", len(c.particles))
	return nil
}

// Register handles particle registration.
func (c *Controller) Register(ctx context.Context, in RegisterInput) (RegisterOutput, error) {
	swarmID := c.cfg.Swarm.ClusterID

	c.mu.Lock()
	defer c.mu.Unlock()

	// idempotent: if already registered, return existing entry
	if _, exists := c.particles[in.ID]; exists {
		return RegisterOutput{ParticleID: in.ID, SwarmSize: c.cfg.Swarm.Size}, nil
	}

	p := pso.NewParticle(in.ID, len(c.cfg.Space))
	c.seedPosition(p, in.Defaults)

	// seed gBest from first particle so social term doesn't pull toward origin
	if c.swarm.GBestFitness <= -1e17 {
		c.swarm.GBest = append([]float64(nil), p.Position...)
	}

	meta := store.ParticleMeta{
		ClientID:       in.ID,
		HealthcheckURL: in.HealthcheckURL,
		ConfigKeys:     in.ConfigKeys,
		Defaults:       in.Defaults,
	}

	if err := c.st.RegisterParticle(ctx, swarmID, in.ID, meta); err != nil {
		return RegisterOutput{}, err
	}
	if err := c.st.AddToRoster(ctx, swarmID, in.ID); err != nil {
		return RegisterOutput{}, err
	}
	if err := c.st.SaveParticle(ctx, swarmID, in.ID, toParticleState(p, "active", 0, 0)); err != nil {
		return RegisterOutput{}, err
	}

	c.particles[in.ID] = p
	c.particleMeta[in.ID] = meta
	c.particleState[in.ID] = "active"
	c.skipCounts[in.ID] = 0
	c.failureCounts[in.ID] = 0

	slog.Info("particle registered", "swarm_id", swarmID, "particle_id", in.ID)
	return RegisterOutput{ParticleID: in.ID, SwarmSize: c.cfg.Swarm.Size}, nil
}

// seedPosition sets p.Position based on new_particle_seed strategy.
// Must be called with c.mu held.
func (c *Controller) seedPosition(p *pso.Particle, defaults map[string]float64) {
	switch c.cfg.Swarm.NewParticleSeed {
	case "gbest":
		if c.swarm.GBestFitness > -1e17 {
			copy(p.Position, c.swarm.GBest)
			copy(p.PBest, c.swarm.GBest)
			return
		}
		fallthrough
	case "client":
		for i, dim := range c.cfg.Space {
			if v, ok := defaults[dim.Name]; ok {
				p.Position[i] = clampDim(v, dim)
			} else {
				p.Position[i] = clampDim(dim.Default, dim)
			}
		}
		copy(p.PBest, p.Position)
	case "random":
		for i, dim := range c.cfg.Space {
			p.Position[i] = dim.Min + c.rng.Float64()*(dim.Max-dim.Min)
			if dim.Type == pso.DimInt {
				p.Position[i] = float64(int(p.Position[i] + 0.5))
			}
		}
		copy(p.PBest, p.Position)
	}
}

// Config returns the current position for a particle as a config map.
func (c *Controller) Config(_ context.Context, particleID string) (ConfigOutput, error) {
	c.mu.RLock()
	p, ok := c.particles[particleID]
	iter := c.currentIter
	c.mu.RUnlock()

	if !ok {
		return ConfigOutput{}, fmt.Errorf("unknown particle %q", particleID)
	}

	cfg := make(map[string]float64, len(c.cfg.Space))
	for i, dim := range c.cfg.Space {
		cfg[dim.Name] = p.Position[i]
	}

	return ConfigOutput{
		Iteration:         iter,
		ObservationWindow: c.cfg.Swarm.ObservationWindow.String(),
		Config:            cfg,
	}, nil
}

// SubmitReport records a particle's metrics for the current iteration.
// Returns O(1) — does not block on iteration advancement.
func (c *Controller) SubmitReport(_ context.Context, particleID string, in ReportInput) (ReportOutput, error) {
	c.mu.RLock()
	_, ok := c.particles[particleID]
	curIter := c.currentIter
	b := c.barrier
	c.mu.RUnlock()

	if !ok {
		return ReportOutput{}, fmt.Errorf("unknown particle %q", particleID)
	}
	if in.Iteration != curIter {
		return ReportOutput{Status: "stale"}, nil
	}

	c.mu.Lock()
	c.pendingReports[particleID] = reportEntry{metrics: in.Metrics}
	c.mu.Unlock()

	if b != nil {
		b.Report(particleID)
	}

	slog.Info("report received", "swarm_id", c.cfg.Swarm.ClusterID, "particle_id", particleID, "iteration", curIter)
	return ReportOutput{Status: "received"}, nil
}

// Status returns the current swarm status.
func (c *Controller) Status(_ context.Context) StatusOutput {
	c.mu.RLock()
	iter := c.currentIter
	gbFit := c.swarm.GBestFitness
	gbPos := append([]float64(nil), c.swarm.GBest...)
	c.mu.RUnlock()

	gbCfg := make(map[string]float64, len(c.cfg.Space))
	for i, dim := range c.cfg.Space {
		if i < len(gbPos) {
			gbCfg[dim.Name] = gbPos[i]
		}
	}

	return StatusOutput{
		Iteration:    iter,
		MaxIter:      c.cfg.Swarm.MaxIter,
		Converged:    false,
		GBestFitness: gbFit,
		GBestConfig:  gbCfg,
	}
}

// Run is the main iteration loop. Blocking; call in a goroutine.
func (c *Controller) Run(ctx context.Context) {
	defer close(c.doneCh)
	swarmID := c.cfg.Swarm.ClusterID

	go c.runHealthcheckLoop(ctx)

	for {
		c.mu.RLock()
		iter := c.currentIter
		c.mu.RUnlock()

		if c.cfg.Swarm.MaxIter >= 0 && iter >= c.cfg.Swarm.MaxIter {
			slog.Info("swarm finished", "swarm_id", swarmID, "iteration", iter)
			break
		}

		select {
		case <-c.stopCh:
			slog.Info("shutdown requested", "swarm_id", swarmID, "iteration", iter)
			return
		default:
		}

		activeIDs := c.activeParticleIDs()
		if len(activeIDs) == 0 {
			slog.Info("no active particles, waiting", "swarm_id", swarmID, "iteration", iter)
			select {
			case <-time.After(c.cfg.Swarm.ReportTimeout):
			case <-c.stopCh:
				return
			}
			continue
		}

		// install new barrier for this iteration
		c.mu.Lock()
		c.pendingReports = make(map[string]reportEntry)
		b := newIterBarrier(activeIDs)
		c.barrier = b
		c.mu.Unlock()

		slog.Info("iteration start", "swarm_id", swarmID, "iteration", iter, "active_particles", len(activeIDs))

		// wait for all active particles to report or timeout
		select {
		case <-b.readyCh:
			slog.Info("all particles reported", "swarm_id", swarmID, "iteration", iter)
		case <-time.After(c.cfg.Swarm.ReportTimeout):
			slog.Warn("report timeout", "swarm_id", swarmID, "iteration", iter)
		case <-ctx.Done():
			return
		}

		unreported := b.Unreported(activeIDs)
		c.handleUnreported(swarmID, iter, unreported)

		c.mu.RLock()
		reports := make(map[string]reportEntry, len(c.pendingReports))
		for k, v := range c.pendingReports {
			reports[k] = v
		}
		c.mu.RUnlock()

		scores := c.fanOutFC(ctx, swarmID, iter, reports)

		c.mu.Lock()
		for pid, score := range scores {
			p := c.particles[pid]
			if pso.UpdatePBest(p, score) {
				c.swarm.UpdateGBest(p)
			}
			c.skipCounts[pid] = 0
			c.m.ParticleFitness.WithLabelValues(pid).Set(score)
		}
		for _, pid := range activeIDs {
			if c.particleState[pid] == "active" {
				c.swarm.Update(c.particles[pid], c.rng)
			}
		}
		for pid, p := range c.particles {
			for i, dim := range c.cfg.Space {
				c.m.ParticlePosition.WithLabelValues(pid, dim.Name).Set(p.Position[i])
				c.m.ParticleVelocity.WithLabelValues(pid, dim.Name).Set(p.Velocity[i])
				c.m.ParticlePBest.WithLabelValues(pid, dim.Name).Set(p.PBest[i])
			}
		}
		c.currentIter++
		c.mu.Unlock()

		c.persistIteration(ctx, swarmID, activeIDs)

		c.m.Iteration.Set(float64(c.currentIter))
		c.m.GBestFitness.Set(c.swarm.GBestFitness)

		slog.Info("iteration done", "swarm_id", swarmID, "iteration", iter, "gbest_fitness", c.swarm.GBestFitness)
	}
}

// handleUnreported increments skip counts and marks particles inactive if over limit.
func (c *Controller) handleUnreported(swarmID string, iter int, unreported []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, pid := range unreported {
		c.skipCounts[pid]++
		if c.skipCounts[pid] > c.cfg.Swarm.MaxSkipIterations {
			c.particleState[pid] = "inactive"
			slog.Warn("particle marked inactive", "swarm_id", swarmID, "particle_id", pid, "iteration", iter)
		}
	}
}

// fanOutFC calls the FC for each reporter concurrently and returns particle → score.
func (c *Controller) fanOutFC(ctx context.Context, swarmID string, iter int, reports map[string]reportEntry) map[string]float64 {
	scores := make(map[string]float64, len(reports))
	var mu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)
	for pid, entry := range reports {
		pid, entry := pid, entry
		g.Go(func() error {
			score, err := c.callFC(gctx, pid, iter, entry.metrics)
			if err != nil {
				slog.Warn("FC call failed", "swarm_id", swarmID, "particle_id", pid, "iteration", iter, "err", err)
				c.mu.RLock()
				score = c.swarm.GBestFitness - 1e-9
				c.mu.RUnlock()
			}
			mu.Lock()
			scores[pid] = score
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()
	return scores
}

// persistIteration writes particle states, gBest, and iteration counter to Redis.
func (c *Controller) persistIteration(ctx context.Context, swarmID string, activeIDs []string) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, pid := range activeIDs {
		p := c.particles[pid]
		st := toParticleState(p, c.particleState[pid], c.skipCounts[pid], c.failureCounts[pid])
		if err := c.st.SaveParticle(ctx, swarmID, pid, st); err != nil {
			slog.Warn("save particle failed", "swarm_id", swarmID, "particle_id", pid, "err", err)
		}
	}
	if err := c.st.SaveGBest(ctx, swarmID, c.swarm.GBest, c.swarm.GBestFitness); err != nil {
		slog.Warn("save gbest failed", "swarm_id", swarmID, "err", err)
	}
	if _, err := c.st.IncrIteration(ctx, swarmID); err != nil {
		slog.Warn("incr iteration failed", "swarm_id", swarmID, "err", err)
	}
}

// runHealthcheckLoop pings each active particle's healthcheck URL periodically.
func (c *Controller) runHealthcheckLoop(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.Swarm.ObservationWindow)
	defer ticker.Stop()
	hcClient := &http.Client{Timeout: 5 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.mu.RLock()
			checks := make(map[string]string)
			for pid, state := range c.particleState {
				if state == "active" {
					checks[pid] = c.particleMeta[pid].HealthcheckURL
				}
			}
			swarmID := c.cfg.Swarm.ClusterID
			c.mu.RUnlock()

			for pid, url := range checks {
				if url == "" {
					continue
				}
				resp, err := hcClient.Get(url)
				ok := err == nil && resp.StatusCode < 400
				if resp != nil {
					resp.Body.Close()
				}

				c.mu.Lock()
				if ok {
					c.failureCounts[pid] = 0
				} else {
					c.failureCounts[pid]++
					if c.failureCounts[pid] >= c.cfg.Swarm.MaxReportFailures {
						c.particleState[pid] = "inactive"
						slog.Warn("particle healthcheck failed, marking inactive", "swarm_id", swarmID, "particle_id", pid)
					}
				}
				c.mu.Unlock()
			}
		}
	}
}

// Shutdown signals Run to stop after the current iteration completes.
func (c *Controller) Shutdown() {
	close(c.stopCh)
}

// Done returns a channel closed when Run has fully exited.
func (c *Controller) Done() <-chan struct{} {
	return c.doneCh
}

// activeParticleIDs returns IDs of all active particles.
func (c *Controller) activeParticleIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var ids []string
	for pid, state := range c.particleState {
		if state == "active" {
			ids = append(ids, pid)
		}
	}
	return ids
}

type fcRequest struct {
	ParticleID string                 `json:"particle_id"`
	Iteration  int                    `json:"iteration"`
	Metrics    map[string]interface{} `json:"metrics"`
}

type fcResponse struct {
	Score float64 `json:"score"`
}

func (c *Controller) callFC(ctx context.Context, particleID string, iter int, m map[string]interface{}) (float64, error) {
	body, err := json.Marshal(fcRequest{ParticleID: particleID, Iteration: iter, Metrics: m})
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.cfg.Swarm.FitnessCalculatorURL+"/fitness", strings.NewReader(string(body)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.fcHTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("FC returned %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var fcResp fcResponse
	if err := json.Unmarshal(raw, &fcResp); err != nil {
		return 0, err
	}
	return fcResp.Score, nil
}

func toParticleState(p *pso.Particle, state string, skipCount, failureCount int) store.ParticleState {
	return store.ParticleState{
		Position:     p.Position,
		Velocity:     p.Velocity,
		PBest:        p.PBest,
		PBestFitness: p.PBestFitness,
		State:        state,
		SkipCount:    skipCount,
		FailureCount: failureCount,
	}
}

// clampDim clamps v to [dim.Min, dim.Max] and rounds to int if DimInt.
func clampDim(v float64, dim pso.Dimension) float64 {
	if v < dim.Min {
		v = dim.Min
	}
	if v > dim.Max {
		v = dim.Max
	}
	if dim.Type == pso.DimInt {
		return float64(int(v + 0.5))
	}
	return v
}
