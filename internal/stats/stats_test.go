package stats

import (
	"sync"
	"testing"
)

// TestConcurrentIncrementsRace exercises IncrementCompleted/IncrementFailed
// from many goroutines at once. Before the atomic.Int64 fix, this reliably
// triggered `go test -race` with as few as 2 goroutines when the counters
// were plain `int` fields incremented via `count++` — see project git
// history for the original failing run. It now passes cleanly, and also
// asserts the exact expected count (not just "ran without racing"), since
// a race in the old implementation could silently lose updates and still
// produce a plausible-looking (but wrong) final number.
func TestConcurrentIncrementsRace(t *testing.T) {
	const goroutines = 100
	const incrementsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			for range incrementsPerGoroutine {
				IncrementCompleted()
				IncrementFailed()
			}
		}()
	}

	wg.Wait()

	want := int64(goroutines * incrementsPerGoroutine)
	snapshot := Snapshot()

	if snapshot.JobsCompletedSinceStartup != want {
		t.Errorf("JobsCompletedSinceStartup = %d, want %d (a lower count indicates lost updates from a race)",
			snapshot.JobsCompletedSinceStartup, want)
	}
	if snapshot.JobsFailedSinceStartup != want {
		t.Errorf("JobsFailedSinceStartup = %d, want %d (a lower count indicates lost updates from a race)",
			snapshot.JobsFailedSinceStartup, want)
	}
}
