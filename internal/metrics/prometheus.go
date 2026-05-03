// Package metrics exposes Prometheus instrumentation for the Controller.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all registered Prometheus metrics.
type Metrics struct {
	Iteration    prometheus.Gauge
	GBestFitness prometheus.Gauge
	ParticleFitness *prometheus.GaugeVec
}

// New registers and returns the Controller metrics.
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
	}
	reg.MustRegister(m.Iteration, m.GBestFitness, m.ParticleFitness)
	return m
}

// Handler returns an HTTP handler for /metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}
