// Package metrics exposes Prometheus instrumentation for the Controller.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all registered Prometheus metrics.
type Metrics struct {
	Iteration        prometheus.Gauge
	GBestFitness     prometheus.Gauge
	ParticleFitness  *prometheus.GaugeVec
	ParticlePosition *prometheus.GaugeVec
	ParticleVelocity *prometheus.GaugeVec
	ParticlePBest    *prometheus.GaugeVec
}

// New registers and returns the Controller metrics on the given registry.
func New(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		Iteration: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pso_iteration",
			Help: "Current swarm iteration.",
		}),
		GBestFitness: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pso_gbest_fitness",
			Help: "Global best fitness score.",
		}),
		ParticleFitness: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pso_particle_fitness",
			Help: "Per-particle fitness score.",
		}, []string{"particle_id"}),
		ParticlePosition: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pso_particle_position",
			Help: "Per-particle current position (config value) per dimension.",
		}, []string{"particle_id", "variable"}),
		ParticleVelocity: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pso_particle_velocity",
			Help: "Per-particle velocity per dimension.",
		}, []string{"particle_id", "variable"}),
		ParticlePBest: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "pso_particle_pbest",
			Help: "Per-particle personal best position per dimension.",
		}, []string{"particle_id", "variable"}),
	}
	reg.MustRegister(m.Iteration, m.GBestFitness, m.ParticleFitness,
		m.ParticlePosition, m.ParticleVelocity, m.ParticlePBest)
	return m
}

// Handler returns an HTTP handler for /metrics using the given gatherer.
func Handler(reg prometheus.Gatherer) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
