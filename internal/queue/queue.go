// Package queue owns the bounded channel that connects the HTTP API
// (producer) to the worker pool (consumer). This is deliberately its own
// small package so the "what happens when we're full" decision lives in
// one obvious place.
package queue

import "errors"

var ErrFull = errors.New("job queue is full")

type Queue struct {
	jobs chan string // job IDs
}

// creates a bounded queue with the given capacity
func New(capacity int) *Queue {
	return &Queue{jobs: make(chan string, capacity)}
}

// attempts a non-blocking send
func (q *Queue) Enqueue(jobID string) error {
	select {
	case q.jobs <- jobID:
		return nil
	default:
		return ErrFull
	}
}

// exposes the receive-only side of the channel for workers to range over
func (q *Queue) Jobs() <-chan string {
	return q.jobs
}

func (q *Queue) Close() {
	close(q.jobs)
}
