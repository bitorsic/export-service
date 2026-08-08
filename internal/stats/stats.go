// Package stats tracks simple in-memory, since-startup operational
// counters — how many export jobs have completed or failed while this
// process has been running.
package stats

import "sync/atomic"

type Counters struct {
	JobsCompletedSinceStartup int64
	JobsFailedSinceStartup    int64
}

var (
	completed atomic.Int64
	failed    atomic.Int64
)

func IncrementCompleted() {
	completed.Add(1)
}

func IncrementFailed() {
	failed.Add(1)
}

func Snapshot() Counters {
	return Counters{
		JobsCompletedSinceStartup: completed.Load(),
		JobsFailedSinceStartup:    failed.Load(),
	}
}
