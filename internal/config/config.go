// Package config loads and validates swarm.yaml into typed structs.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/theodore-chandra/pso-config-tuner/internal/pso"
)

// Config is the fully parsed and validated configuration.
type Config struct {
	Swarm SwarmConfig
	PSO   PSOConfig
	Space []pso.Dimension
}

// SwarmConfig holds operational parameters for the swarm.
type SwarmConfig struct {
	ClusterID               string
	Size                    int
	MaxIter                 int
	ObservationWindow       time.Duration
	FitnessCalculatorURL    string
	ReportTimeout           time.Duration
	MaxSkipIterations       int
	EvictionAfterIterations int
	MaxReportFailures       int
	NewParticleSeed         string
}

// PSOConfig holds PSO hyperparameters.
type PSOConfig struct {
	Inertia   float64
	Cognitive float64
	Social    float64
}

// rawConfig mirrors swarm.yaml verbatim before resolution.
type rawConfig struct {
	Swarm struct {
		ClusterID               string `yaml:"cluster_id"`
		Size                    int    `yaml:"size"`
		MaxIter                 int    `yaml:"max_iter"`
		ObservationWindow       string `yaml:"observation_window"`
		FitnessCalculatorURL    string `yaml:"fitness_calculator_url"`
		ReportTimeout           string `yaml:"report_timeout"`
		MaxSkipIterations       int    `yaml:"max_skip_iterations"`
		EvictionAfterIterations int    `yaml:"eviction_after_iterations"`
		MaxReportFailures       int    `yaml:"max_report_failures"`
		NewParticleSeed         string `yaml:"new_particle_seed"`
	} `yaml:"swarm"`
	PSO struct {
		Inertia   float64 `yaml:"inertia"`
		Cognitive float64 `yaml:"cognitive"`
		Social    float64 `yaml:"social"`
	} `yaml:"pso"`
	Space []rawDimension `yaml:"space"`
}

// rawDimension uses interface{} for min/max to handle string cross-param references.
type rawDimension struct {
	Name    string      `yaml:"name"`
	Type    string      `yaml:"type"`
	Min     interface{} `yaml:"min"`
	Max     interface{} `yaml:"max"`
	Default float64     `yaml:"default"`
	Values  []string    `yaml:"values"`
}

// Load reads the YAML file at path, resolves cross-param references, and validates.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	space, err := resolveSpace(raw.Space)
	if err != nil {
		return nil, fmt.Errorf("resolve search space: %w", err)
	}

	ow, err := parseDurationDefault(raw.Swarm.ObservationWindow, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("observation_window: %w", err)
	}
	rt, err := parseDurationDefault(raw.Swarm.ReportTimeout, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("report_timeout: %w", err)
	}

	cfg := &Config{
		Swarm: SwarmConfig{
			ClusterID:               raw.Swarm.ClusterID,
			Size:                    defaultInt(raw.Swarm.Size, 10),
			MaxIter:                 defaultInt(raw.Swarm.MaxIter, 100),
			ObservationWindow:       ow,
			FitnessCalculatorURL:    raw.Swarm.FitnessCalculatorURL,
			ReportTimeout:           rt,
			MaxSkipIterations:       defaultInt(raw.Swarm.MaxSkipIterations, 3),
			EvictionAfterIterations: defaultEviction(raw.Swarm.EvictionAfterIterations),
			MaxReportFailures:       defaultInt(raw.Swarm.MaxReportFailures, 5),
			NewParticleSeed:         defaultStr(raw.Swarm.NewParticleSeed, "client"),
		},
		PSO: PSOConfig{
			Inertia:   defaultFloat(raw.PSO.Inertia, 0.729),
			Cognitive: defaultFloat(raw.PSO.Cognitive, 1.49445),
			Social:    defaultFloat(raw.PSO.Social, 1.49445),
		},
		Space: space,
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// resolveSpace converts raw dimensions to pso.Dimension, resolving string bound references.
// Bounds that reference another param name resolve to that param's Default value.
func resolveSpace(raw []rawDimension) ([]pso.Dimension, error) {
	index := make(map[string]float64, len(raw))
	for _, d := range raw {
		index[d.Name] = d.Default
	}

	dims := make([]pso.Dimension, len(raw))
	for i, d := range raw {
		min, err := resolveBound(d.Min, index)
		if err != nil {
			return nil, fmt.Errorf("dim %q min: %w", d.Name, err)
		}
		max, err := resolveBound(d.Max, index)
		if err != nil {
			return nil, fmt.Errorf("dim %q max: %w", d.Name, err)
		}
		if min >= max {
			return nil, fmt.Errorf("dim %q: min (%v) must be < max (%v)", d.Name, min, max)
		}
		dims[i] = pso.Dimension{
			Name:    d.Name,
			Type:    pso.DimType(d.Type),
			Min:     min,
			Max:     max,
			Default: d.Default,
			Values:  d.Values,
		}
	}
	return dims, nil
}

// resolveBound converts a raw YAML bound value to float64.
// Strings are treated as references to another dimension's Default.
func resolveBound(v interface{}, index map[string]float64) (float64, error) {
	switch t := v.(type) {
	case float64:
		return t, nil
	case int:
		return float64(t), nil
	case string:
		val, ok := index[t]
		if !ok {
			return 0, fmt.Errorf("unknown param reference %q", t)
		}
		return val, nil
	case nil:
		return 0, fmt.Errorf("bound is required")
	default:
		return 0, fmt.Errorf("unexpected bound type %T", v)
	}
}

func validate(cfg *Config) error {
	if cfg.Swarm.FitnessCalculatorURL == "" {
		return fmt.Errorf("fitness_calculator_url is required")
	}
	if cfg.Swarm.Size < 1 {
		return fmt.Errorf("swarm.size must be >= 1")
	}
	if len(cfg.Space) == 0 {
		return fmt.Errorf("search space must have at least one dimension")
	}
	seed := cfg.Swarm.NewParticleSeed
	if seed != "client" && seed != "gbest" && seed != "random" {
		return fmt.Errorf("new_particle_seed must be client|gbest|random, got %q", seed)
	}
	return nil
}

func parseDurationDefault(s string, def time.Duration) (time.Duration, error) {
	if s == "" {
		return def, nil
	}
	return time.ParseDuration(s)
}

func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func defaultFloat(v, def float64) float64 {
	if v == 0 {
		return def
	}
	return v
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// defaultEviction: 0 in YAML means unset; -1 means never (the intended default).
func defaultEviction(v int) int {
	if v == 0 {
		return -1
	}
	return v
}
