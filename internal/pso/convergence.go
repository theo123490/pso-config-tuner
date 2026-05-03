package pso

// ConvergenceChecker tracks whether gBest has improved enough over recent iterations.
type ConvergenceChecker struct {
	Threshold float64 // minimum improvement to count as progress
	Patience  int     // iterations without progress before declaring convergence
	stale     int
	lastBest  float64
}

// NewConvergenceChecker creates a checker with the given threshold and patience.
func NewConvergenceChecker(threshold float64, patience int) *ConvergenceChecker {
	return &ConvergenceChecker{
		Threshold: threshold,
		Patience:  patience,
		lastBest:  -1e18,
	}
}

// Record reports the current gBest fitness. Returns true if converged.
func (c *ConvergenceChecker) Record(gBestFitness float64) bool {
	if gBestFitness-c.lastBest >= c.Threshold {
		c.stale = 0
		c.lastBest = gBestFitness
	} else {
		c.stale++
	}
	return c.stale >= c.Patience
}
