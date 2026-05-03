# PSO Config Tuner

Automatic microservice configuration tuner using Particle Swarm Optimization (PSO). Deployed as a set of Kubernetes services — plug your microservice in as a particle, define a fitness function, let the swarm find optimal configuration parameters.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Kubernetes Swarm Deployment                                │
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │  Controller  │◄──►│    Redis     │    │  Prometheus  │  │
│  │  (Go)        │    │  (state)     │    │  + Grafana   │  │
│  └──────┬───────┘    └──────────────┘    └──────────────┘  │
│         │                                                   │
│         ├──► POST /fitness ──► Fitness Calculator (any)     │
│         │                                                   │
│         ├──► GET /config ◄──┐                               │
│         └──► POST /report ◄─┤  Client × N (any language)   │
│                             └──────────────────────────────  │
└─────────────────────────────────────────────────────────────┘
```

| Component | Language | Role |
|-----------|----------|------|
| **Controller** | Go | Orchestrates PSO swarm. REST API. Redis state. |
| **Client** | Any | Your microservice. Pulls config, applies it, reports metrics. |
| **Fitness Calculator** | Any | Stateless scorer. Receives metrics, returns `float64` score. |

## How it works

1. Define your search space in `configs/swarm.yaml` (parameter names, types, bounds).
2. Deploy the Controller, your Fitness Calculator, and N replicas of your microservice (Client).
3. Clients register with the Controller on startup.
4. Each iteration: clients pull a new config, apply it, observe for a window, report metrics.
5. Controller calls the Fitness Calculator, updates pBest/gBest, computes new positions via PSO.
6. Repeat until convergence or `max_iter`. Read optimal config from `GET /status`.

**Fitness convention: higher score = better.** Negate if you want to minimize latency.

## Quick start

```bash
# Edit the search space and PSO settings
vim configs/swarm.yaml

# Start controller + Redis via Docker Compose
make up

# Check swarm status
curl http://localhost:8080/status
```

## Local development

| Command | Description |
|---------|-------------|
| `make build` | Compile binary to `bin/controller` |
| `make run` | Build and run the controller locally |
| `make up` | Start controller + Redis with Docker Compose |
| `make down` | Stop Docker Compose services |
| `make logs` | Tail controller logs |
| `make test` | Run all tests |
| `make lint` | Run `go vet` |
| `make tidy` | Run `go mod tidy` |

The controller listens on `:8080` by default. Override with `CONTROLLER_ADDR` env var.

## Configuration (`swarm.yaml`)

```yaml
swarm:
  cluster_id: my-swarm-001        # used as auth token during registration
  size: 10                         # number of particles
  max_iter: 100
  observation_window: 30s          # sent to clients via /config response
  fitness_calculator_url: http://fitness-calc:9000
  report_timeout: 5s
  max_skip_iterations: 3
  eviction_after_iterations: -1    # -1 = never evict
  max_report_failures: 5
  new_particle_seed: client        # gbest | random | client

pso:
  inertia: 0.729
  cognitive: 1.49445
  social: 1.49445
  convergence_threshold: 1e-4
  convergence_patience: 10

space:
  - name: worker_pool_size
    type: int
    min: 1
    max: 64
    default: 8
  - name: queue_depth
    type: int
    min: worker_pool_size          # cross-parameter bound reference
    max: 10000
    default: 500
  - name: timeout_ms
    type: float
    min: 50.0
    max: 5000.0
    default: 1000.0
```

## REST API

### Controller

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/register` | Register a particle |
| `GET` | `/config/{particle_id}` | Pull current config for iteration |
| `POST` | `/report/{particle_id}` | Submit observation metrics |
| `GET` | `/status` | Swarm status, gBest config |
| `GET` | `/metrics` | Prometheus metrics |

### Fitness Calculator contract

```
POST /fitness
{ "particle_id": "...", "iteration": 4, "metrics": { ... } }

→ 200 { "score": 0.874 }
```

Implement this endpoint in any language and point `fitness_calculator_url` at it.

## Project structure

```
pso-config-tuner/
├── cmd/controller/         # main entry point
├── internal/
│   ├── pso/                # pure PSO algorithm (no I/O, stdlib only)
│   ├── api/                # HTTP handlers
│   ├── store/              # Redis client
│   └── metrics/            # Prometheus instrumentation
├── configs/
│   └── swarm.yaml
├── examples/
│   ├── client-go/          # example Go client
│   └── fitness-python/     # example Python fitness calculator
└── deploy/
    └── k8s/                # Kubernetes manifests
```

## Milestones

| M | Scope |
|---|-------|
| M1 | Core PSO algorithm (`internal/pso`) |
| M2 | Redis store (`internal/store`) |
| M3 | Controller REST API |
| M4 | Iteration barrier + health-check loop |
| M5 | Fitness Calculator integration |
| M6 | YAML config + Prometheus + structured logging |
| M7 | Convergence detection + graceful shutdown |
| M8 | Examples + Kubernetes manifests |

## References

- Kennedy & Eberhart (1995). *Particle Swarm Optimization*. ICNN.
- Clerc & Kennedy (2002). *The Particle Swarm — Explosion, Stability, and Convergence*. IEEE TEC.
