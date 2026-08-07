package stats

import (
	"sync"
	"testing"
)

func TestConcurrentIncrementsRace(t *testing.T) {
	current = &Counters{}

	const goroutines = 2
	const incrementsPerGoroutine = 1

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

	snapshot := Snapshot()
	if snapshot.JobsCompletedSinceStartup == 0 || snapshot.JobsFailedSinceStartup == 0 {
		t.Fatalf("expected increments to run, got %#v", snapshot)
	}
}
