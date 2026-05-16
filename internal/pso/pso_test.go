package pso_test

import (
	"math/rand"
	"testing"

	"github.com/theodore-chandra/pso-config-tuner/internal/pso"
)

func dims2D() []pso.Dimension {
	return []pso.Dimension{
		{Name: "x", Type: pso.DimFloat, Min: 0, Max: 10},
		{Name: "y", Type: pso.DimFloat, Min: 0, Max: 10},
	}
}

func TestNewParticle(t *testing.T) {
	p := pso.NewParticle("p1", 3)
	if p.ID != "p1" {
		t.Fatalf("want ID=p1, got %q", p.ID)
	}
	if len(p.Position) != 3 || len(p.Velocity) != 3 || len(p.PBest) != 3 {
		t.Fatalf("wrong slice lengths: pos=%d vel=%d pbest=%d", len(p.Position), len(p.Velocity), len(p.PBest))
	}
	if p.PBestFitness != -1e18 {
		t.Fatalf("want PBestFitness=-1e18, got %v", p.PBestFitness)
	}
}

func TestUpdatePBest_Improvement(t *testing.T) {
	p := pso.NewParticle("p1", 2)
	p.Position = []float64{3.0, 4.0}

	if !pso.UpdatePBest(p, 5.0) {
		t.Fatal("expected improvement")
	}
	if p.PBestFitness != 5.0 {
		t.Fatalf("want 5.0, got %v", p.PBestFitness)
	}
	if p.PBest[0] != 3.0 || p.PBest[1] != 4.0 {
		t.Fatalf("pbest not copied from position: %v", p.PBest)
	}
}

func TestUpdatePBest_NoImprovement(t *testing.T) {
	p := pso.NewParticle("p1", 2)
	pso.UpdatePBest(p, 5.0)

	if pso.UpdatePBest(p, 3.0) {
		t.Fatal("should not improve on worse fitness")
	}
	if p.PBestFitness != 5.0 {
		t.Fatalf("pbest should stay 5.0, got %v", p.PBestFitness)
	}
}

func TestUpdateGBest_Improvement(t *testing.T) {
	s := pso.NewSwarm(dims2D())
	p := pso.NewParticle("p1", 2)
	p.PBest = []float64{1.0, 2.0}
	p.PBestFitness = 5.0

	if !s.UpdateGBest(p) {
		t.Fatal("expected gbest update")
	}
	if s.GBestFitness != 5.0 {
		t.Fatalf("want 5.0, got %v", s.GBestFitness)
	}
	if s.GBest[0] != 1.0 || s.GBest[1] != 2.0 {
		t.Fatalf("gbest position wrong: %v", s.GBest)
	}
}

func TestUpdateGBest_NoImprovement(t *testing.T) {
	s := pso.NewSwarm(dims2D())
	p1 := pso.NewParticle("p1", 2)
	p1.PBest = []float64{1.0, 2.0}
	p1.PBestFitness = 5.0
	s.UpdateGBest(p1)

	p2 := pso.NewParticle("p2", 2)
	p2.PBest = []float64{9.0, 9.0}
	p2.PBestFitness = 3.0

	if s.UpdateGBest(p2) {
		t.Fatal("should not update gbest on worse fitness")
	}
	if s.GBestFitness != 5.0 {
		t.Fatal("gbest should remain 5.0")
	}
}

func TestSwarmUpdate_ClampsToBounds(t *testing.T) {
	dims := []pso.Dimension{
		{Name: "x", Type: pso.DimFloat, Min: 0, Max: 1},
	}
	s := pso.NewSwarm(dims)
	s.GBest = []float64{1.0}
	s.GBestFitness = 10.0
	s.Inertia = 100.0

	p := pso.NewParticle("p1", 1)
	p.Position = []float64{0.5}
	p.Velocity = []float64{999.0}
	p.PBest = []float64{0.5}

	s.Update(p, rand.New(rand.NewSource(42)))

	if p.Position[0] < dims[0].Min || p.Position[0] > dims[0].Max {
		t.Fatalf("position %v outside bounds [%v, %v]", p.Position[0], dims[0].Min, dims[0].Max)
	}
}

func TestSwarmUpdate_IntRounding(t *testing.T) {
	dims := []pso.Dimension{
		{Name: "n", Type: pso.DimInt, Min: 1, Max: 10},
	}
	s := pso.NewSwarm(dims)
	s.GBest = []float64{5.0}
	s.GBestFitness = 1.0
	s.Inertia = 0.1

	p := pso.NewParticle("p1", 1)
	p.Position = []float64{3.0}
	p.PBest = []float64{3.0}
	p.PBestFitness = 1.0

	s.Update(p, rand.New(rand.NewSource(0)))

	if p.Position[0] != float64(int(p.Position[0])) {
		t.Fatalf("DimInt position %v is not integer-valued", p.Position[0])
	}
}

