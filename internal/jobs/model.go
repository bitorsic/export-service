// Package jobs defines the export job model and PostgreSQL-backed
// data access for it. Both the HTTP API (creating/reading jobs) and the
// worker pool (updating job status) depend on this package.
package jobs

import (
	"time"
)

// Status is the lifecycle state of an export job.
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

// Job mirrors a row in the export_jobs table.
type Job struct {
	ID        string
	Status    Status
	SellerID  string
	DateFrom  time.Time
	DateTo    time.Time
	FilePath  *string
	Error     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}
