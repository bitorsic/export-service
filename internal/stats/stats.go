// Package stats tracks simple in-memory, since-startup operational
// counters — how many export jobs have completed or failed while this
// process has been running.
package stats

type Counters struct {
	JobsCompletedSinceStartup int
	JobsFailedSinceStartup    int
}

var current = &Counters{}

func IncrementCompleted() {
	current.JobsCompletedSinceStartup++
}

func IncrementFailed() {
	current.JobsFailedSinceStartup++
}
func Snapshot() Counters {
	return *current
}
