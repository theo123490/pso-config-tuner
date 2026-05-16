package controller

import "sync"

// iterBarrier is a per-iteration countdown latch.
// readyCh is closed exactly once when all expected particles have reported.
type iterBarrier struct {
	mu       sync.Mutex
	expected int
	count    int
	reported map[string]struct{}
	readyCh  chan struct{}
	once     sync.Once
}

func newIterBarrier(activeIDs []string) *iterBarrier {
	b := &iterBarrier{
		expected: len(activeIDs),
		reported: make(map[string]struct{}, len(activeIDs)),
		readyCh:  make(chan struct{}),
	}
	if len(activeIDs) == 0 {
		close(b.readyCh)
	}
	return b
}

// Report records that particleID has reported. Returns true if already reported (idempotent).
// Closes readyCh exactly once when count reaches expected.
func (b *iterBarrier) Report(particleID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.reported[particleID]; ok {
		return true
	}
	b.reported[particleID] = struct{}{}
	b.count++
	if b.count >= b.expected {
		b.once.Do(func() { close(b.readyCh) })
	}
	return false
}

// Unreported returns IDs from activeIDs that have not yet reported.
func (b *iterBarrier) Unreported(activeIDs []string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, id := range activeIDs {
		if _, ok := b.reported[id]; !ok {
			out = append(out, id)
		}
	}
	return out
}
