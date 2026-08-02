package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// wraps the connection pool with export_jobs-specific queries.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// inserts a new pending job and returns its generated ID.
func (s *Store) Create(ctx context.Context, sellerID string, dateFrom, dateTo time.Time) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO export_jobs (status, seller_id, date_from, date_to)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, StatusPending, sellerID, dateFrom, dateTo).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("inserting job: %w", err)
	}
	return id, nil
}

// fetches a single job by ID.
func (s *Store) Get(ctx context.Context, id string) (*Job, error) {
	var j Job
	err := s.pool.QueryRow(ctx, `
		SELECT id, status, seller_id, date_from, date_to, file_path, error, created_at, updated_at
		FROM export_jobs
		WHERE id = $1
	`, id).Scan(&j.ID, &j.Status, &j.SellerID, &j.DateFrom, &j.DateTo, &j.FilePath, &j.Error, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // "not found"
		}
		return nil, fmt.Errorf("fetching job %s: %w", id, err)
	}
	return &j, nil
}

// transitions a job to "processing".
func (s *Store) MarkProcessing(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE export_jobs SET status = $1, updated_at = now() WHERE id = $2
	`, StatusProcessing, id)
	if err != nil {
		return fmt.Errorf("marking job %s processing: %w", id, err)
	}
	return nil
}

// transitions a job to "done" and records the output file path.
func (s *Store) MarkDone(ctx context.Context, id, filePath string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE export_jobs SET status = $1, file_path = $2, updated_at = now() WHERE id = $3
	`, StatusDone, filePath, id)
	if err != nil {
		return fmt.Errorf("marking job %s done: %w", id, err)
	}
	return nil
}

// removes a job row. used to roll back a job that was written to
// Postgres but never made it onto the processing queue
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM export_jobs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting job %s: %w", id, err)
	}
	return nil
}

// transitions a job to "failed" and records the error message.
func (s *Store) MarkFailed(ctx context.Context, id string, cause error) error {
	msg := cause.Error()
	_, err := s.pool.Exec(ctx, `
		UPDATE export_jobs SET status = $1, error = $2, updated_at = now() WHERE id = $3
	`, StatusFailed, msg, id)
	if err != nil {
		return fmt.Errorf("marking job %s failed: %w", id, err)
	}
	return nil
}
