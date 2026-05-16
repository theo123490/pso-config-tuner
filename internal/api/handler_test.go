package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/theodore-chandra/pso-config-tuner/internal/api"
	"github.com/theodore-chandra/pso-config-tuner/internal/config"
	"github.com/theodore-chandra/pso-config-tuner/internal/controller"
	"github.com/theodore-chandra/pso-config-tuner/internal/metrics"
	"github.com/theodore-chandra/pso-config-tuner/internal/pso"
	"github.com/theodore-chandra/pso-config-tuner/internal/store"
)

// mockStore implements store.Store in-memory.
type mockStore struct {
	particles map[string]store.ParticleState
	metas     map[string]store.ParticleMeta
	roster    map[string][]string
	gbestPos  []float64
	gbestFit  float64
	iteration int64
}

func newMockStore() *mockStore {
	return &mockStore{
		particles: make(map[string]store.ParticleState),
		metas:     make(map[string]store.ParticleMeta),
		roster:    make(map[string][]string),
	}
}

func (m *mockStore) SaveParticle(_ context.Context, _, pid string, p store.ParticleState) error {
	m.particles[pid] = p
	return nil
}
func (m *mockStore) LoadParticle(_ context.Context, _, pid string) (store.ParticleState, error) {
	return m.particles[pid], nil
}
func (m *mockStore) SaveGBest(_ context.Context, _ string, pos []float64, fit float64) error {
	m.gbestPos = pos
	m.gbestFit = fit
	return nil
}
func (m *mockStore) LoadGBest(_ context.Context, _ string) ([]float64, float64, error) {
	return m.gbestPos, m.gbestFit, nil
}
func (m *mockStore) IncrIteration(_ context.Context, _ string) (int64, error) {
	m.iteration++
	return m.iteration, nil
}
func (m *mockStore) LoadIteration(_ context.Context, _ string) (int64, error) {
	return m.iteration, nil
}
func (m *mockStore) RegisterParticle(_ context.Context, _, pid string, meta store.ParticleMeta) error {
	m.metas[pid] = meta
	return nil
}
func (m *mockStore) LoadParticleMeta(_ context.Context, _, pid string) (store.ParticleMeta, error) {
	return m.metas[pid], nil
}
func (m *mockStore) AddToRoster(_ context.Context, swarmID, pid string) error {
	m.roster[swarmID] = append(m.roster[swarmID], pid)
	return nil
}
func (m *mockStore) LoadRoster(_ context.Context, swarmID string) ([]string, error) {
	return m.roster[swarmID], nil
}

func testCfg() *config.Config {
	return &config.Config{
		Swarm: config.SwarmConfig{
			ClusterID:               "test-swarm",
			Size:                    5,
			MaxIter:                 100,
			ObservationWindow:       30 * time.Second,
			FitnessCalculatorURL:    "http://fc:9000",
			ReportTimeout:           5 * time.Second,
			MaxSkipIterations:       3,
			EvictionAfterIterations: -1,
			MaxReportFailures:       5,
			NewParticleSeed:         "client",
		},
		PSO: config.PSOConfig{
			Inertia:   0.729,
			Cognitive: 1.49445,
			Social:    1.49445,
		},
		Space: []pso.Dimension{
			{Name: "workers", Type: pso.DimInt, Min: 1, Max: 16, Default: 4},
		},
	}
}

func newTestServer(t *testing.T) (*httptest.Server, *mockStore) {
	t.Helper()
	st := newMockStore()
	reg := prometheus.NewRegistry()
	m := metrics.New(reg)
	ctrl, err := controller.New(testCfg(), st, m)
	if err != nil {
		t.Fatalf("controller.New: %v", err)
	}
	h := api.NewHandler(ctrl)
	mux := http.NewServeMux()
	h.Register(mux)
	return httptest.NewServer(mux), st
}

func postJSON(t *testing.T, url string, body interface{}) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestHandleRegister_HappyPath(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp := postJSON(t, srv.URL+"/register", api.RegisterRequest{
		ID:         "p1",
		ConfigKeys: []string{"workers"},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var out api.RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.ParticleID != "p1" {
		t.Fatalf("want ParticleID=p1, got %q", out.ParticleID)
	}
}

func TestHandleRegister_MissingID(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp := postJSON(t, srv.URL+"/register", api.RegisterRequest{})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestHandleRegister_Idempotent(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	req := api.RegisterRequest{ID: "p1", ConfigKeys: []string{"workers"}}
	resp1 := postJSON(t, srv.URL+"/register", req)
	resp1.Body.Close()
	resp2 := postJSON(t, srv.URL+"/register", req)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("re-register want 200, got %d", resp2.StatusCode)
	}
}

func TestHandleConfig_HappyPath(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	postJSON(t, srv.URL+"/register", api.RegisterRequest{
		ID:         "p1",
		ConfigKeys: []string{"workers"},
	}).Body.Close()

	resp, err := http.Get(srv.URL + "/config/p1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var out api.ConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if _, ok := out.Config["workers"]; !ok {
		t.Fatal("config missing 'workers' key")
	}
}

func TestHandleConfig_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/config/ghost")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestHandleReport_HappyPath(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	postJSON(t, srv.URL+"/register", api.RegisterRequest{
		ID:         "p1",
		ConfigKeys: []string{"workers"},
	}).Body.Close()

	resp := postJSON(t, srv.URL+"/report/p1", api.ReportRequest{
		Iteration: 0,
		Metrics:   map[string]interface{}{"rps": 1200},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var out api.ReportResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Status != "received" {
		t.Fatalf("want status=received, got %q", out.Status)
	}
}

func TestHandleReport_UnknownParticle(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp := postJSON(t, srv.URL+"/report/ghost", api.ReportRequest{
		Iteration: 0,
		Metrics:   map[string]interface{}{},
	})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
}

func TestHandleStatus(t *testing.T) {
	srv, _ := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var out api.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.MaxIter != 100 {
		t.Fatalf("want MaxIter=100, got %d", out.MaxIter)
	}
}
