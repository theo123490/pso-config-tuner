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
6. Repeat until `max_iter` is reached (`-1` = runs indefinitely). Read optimal config from `GET /status`.

**Fitness convention: higher score = better.** Negate if you want to minimize latency.

## Quick start (local)

```bash
# Start the Ackley simulation
cd simulations/ackley
make docker/up

# Check swarm status — iteration should advance within ~10s
curl http://localhost:8080/status

# Grafana dashboards
open http://localhost:3000

# Stop
make docker/down
```

To run a different simulation, `cd` into its directory:

```bash
cd simulations/kafka-consumer
make docker/up
make docker/down
```

## Simulations

Each simulation lives in `simulations/<name>/` and is self-contained:

```
simulations/
├── ackley/               # 2D Ackley function benchmark (active)
│   ├── docker-compose.yml
│   ├── configs/swarm.yaml
│   ├── fitness/          # Python Ackley fitness calculator
│   └── monitoring/       # Prometheus + Grafana provisioning
└── kafka-consumer/       # Kafka consumer config tuning (skeleton — not started)
    ├── docker-compose.yml
    ├── configs/swarm.yaml
    └── fitness/          # stub fitness calculator
```

The Go controller and client (`examples/client-go/`) are shared across all simulations.

### Ackley services

| Service | Port | Notes |
|---------|------|-------|
| controller | 8080 | REST API |
| redis | 6380 | Host port 6380 (avoids conflict with system Redis on 6379) |
| fitness-calc | 9000 | Ackley function scorer; `/render` serves interactive 3D plot |
| client-0 … client-4 | — | 5 Go clients as particles |
| prometheus | 9090 | Scrapes controller every 5s |
| grafana | 3000 | Anonymous admin; PSO Swarm + Ackley Particle Detail dashboards |

### Makefile targets

Run from the simulation directory (`cd simulations/ackley`):

| Command | Description |
|---------|-------------|
| `make docker/up` | Build and start simulation stack (detached) |
| `make docker/down` | Stop and remove simulation containers |
| `make docker/logs` | Tail controller logs |
| `make docker/restart-simulation` | Flush Redis + rebuild/restart controller, fitness-calc, all clients |
| `make docker/restart-fitness` | Rebuild/restart fitness-calc only (no Redis flush) |

Run from the repo root:

| Command | Description |
|---------|-------------|
| `make build` | Compile controller binary (requires Go 1.23+) |
| `make test` | Run all tests |
| `make lint` | Run `go vet` |
| `make tidy` | Run `go mod tidy` |

**Note:** `go.mod` requires Go 1.23. Build always via Docker — the dev stack uses `golang:1.23-alpine`.

If a run gets stuck, flush Redis:

```bash
make docker/restart-simulation
```

The controller listens on `:8080` by default. Override with `CONTROLLER_ADDR` env var.

## Production deployment

Production is deployed on **Kubernetes**, not Docker Compose. Manifests are in `deploy/k8s/`:

| Manifest | Description |
|----------|-------------|
| `namespace.yaml` | `pso-tuner` namespace |
| `configmap.yaml` | `swarm.yaml` as a ConfigMap |
| `redis.yaml` | Redis Deployment + Service |
| `controller-deployment.yaml` | Controller Deployment + readiness probe |
| `controller-service.yaml` | Controller Service |
| `fitness-deployment.yaml` | Fitness Calculator Deployment + Service |
| `client-deployment.yaml` | Client Deployment (N replicas; `PARTICLE_ID` from pod name) |

```bash
kubectl apply -f deploy/k8s/
```

Docker Compose is not used in production.

## Configuration (`swarm.yaml`)

```yaml
swarm:
  cluster_id: my-swarm-001        # used as auth token during registration
  size: 5                          # number of particles
  max_iter: -1                     # -1 = run indefinitely
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

space:
  - name: x1
    type: float
    min: -10.0
    max: 10.0
    default: 2.5
  - name: x2
    type: float
    min: -10.0
    max: 10.0
    default: 2.5
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

The example Python FC (`examples/fitness-python`) implements the 2D Ackley function and also exposes:

```
GET /render              — interactive 3D surface plot of the Ackley function
GET /render?x1=0&x2=0   — same plot with a query point marked
```

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
