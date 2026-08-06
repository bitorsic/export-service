// Package worker implements the bounded pool of goroutines that consume
// job IDs from the queue and turn them into finished CSV exports.
package worker

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bitorsic/export-service/internal/jobs"
	"github.com/bitorsic/export-service/internal/queue"
)

type Pool struct {
	numWorkers int
	queue      *queue.Queue
	store      *jobs.Store
	pool       *pgxpool.Pool
}

func NewPool(numWorkers int, q *queue.Queue, store *jobs.Store, dbPool *pgxpool.Pool) *Pool {
	return &Pool{
		numWorkers: numWorkers,
		queue:      q,
		store:      store,
		pool:       dbPool,
	}
}

// launches the fixed set of worker goroutines and blocks until all of them have exited
//
// when ctx is cancelled, workers stop picking up NEW
// jobs, but a job already in progress is allowed to finish
func (p *Pool) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < p.numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			p.runWorker(ctx, workerID)
		}(i)
	}
	wg.Wait()
	log.Println("[worker] all workers exited")
}

func (p *Pool) runWorker(ctx context.Context, id int) {
	log.Printf("[worker %d] started", id)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[worker %d] shutting down (no new jobs)", id)
			return
		case jobID, ok := <-p.queue.Jobs():
			if !ok {
				log.Printf("[worker %d] queue closed, exiting", id)
				return
			}
			p.process(ctx, id, jobID)
		}
	}
}

func (p *Pool) process(ctx context.Context, workerID int, jobID string) {
	log.Printf("[worker %d] picked up job %s", workerID, jobID)

	job, err := p.store.Get(ctx, jobID)
	if err != nil || job == nil {
		log.Printf("[worker %d] could not load job %s: %v", workerID, jobID, err)
		return
	}

	if err := p.store.MarkProcessing(ctx, jobID); err != nil {
		log.Printf("[worker %d] failed to mark job %s processing: %v", workerID, jobID, err)
		return
	}

	filePath, err := runExport(ctx, p.pool, job)
	if err != nil {
		if errors.Is(err, ErrNoData) {
			log.Printf("[worker %d] job %s: no data for given seller/date range", workerID, jobID)
		} else {
			log.Printf("[worker %d] export failed for job %s: %v", workerID, jobID, err)
		}
		if markErr := p.store.MarkFailed(ctx, jobID, err); markErr != nil {
			log.Printf("[worker %d] additionally failed to mark job %s failed: %v", workerID, jobID, markErr)
		}
		return
	}

	if err := p.store.MarkDone(ctx, jobID, filePath); err != nil {
		log.Printf("[worker %d] failed to mark job %s done: %v", workerID, jobID, err)
		return
	}

	log.Printf("[worker %d] completed job %s -> %s", workerID, jobID, filePath)
}
