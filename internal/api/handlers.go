package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/bitorsic/export-service/internal/jobs"
	"github.com/bitorsic/export-service/internal/queue"
)

type Handler struct {
	store *jobs.Store
	queue *queue.Queue
}

func NewHandler(store *jobs.Store, q *queue.Queue) *Handler {
	return &Handler{store: store, queue: q}
}

type createExportRequest struct {
	SellerID string `json:"seller_id"`
	DateFrom string `json:"date_from"` // "YYYY-MM-DD"
	DateTo   string `json:"date_to"`
}

type createExportResponse struct {
	JobID string `json:"job_id"`
}

func (h *Handler) CreateExport(w http.ResponseWriter, r *http.Request) {
	var req createExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SellerID == "" {
		writeError(w, http.StatusBadRequest, "seller_id is required")
		return
	}

	dateFrom, err := time.Parse("2006-01-02", req.DateFrom)
	if err != nil {
		writeError(w, http.StatusBadRequest, "date_from must be YYYY-MM-DD")
		return
	}
	dateTo, err := time.Parse("2006-01-02", req.DateTo)
	if err != nil {
		writeError(w, http.StatusBadRequest, "date_to must be YYYY-MM-DD")
		return
	}
	if dateTo.Before(dateFrom) {
		writeError(w, http.StatusBadRequest, "date_to must not be before date_from")
		return
	}

	jobID, err := h.store.Create(r.Context(), req.SellerID, dateFrom, dateTo)
	if err != nil {
		log.Printf("[api] failed to create job: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create export job")
		return
	}

	if err := h.queue.Enqueue(jobID); err != nil {
		if errors.Is(err, queue.ErrFull) {
			// backpressure path
			if delErr := h.store.Delete(r.Context(), jobID); delErr != nil {
				log.Printf("[api] failed to roll back orphaned job %s: %v", jobID, delErr)
			}
			writeError(w, http.StatusServiceUnavailable, "system is busy, please retry shortly")
			return
		}
		log.Printf("[api] unexpected queue error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to queue export job")
		return
	}

	writeJSON(w, http.StatusAccepted, createExportResponse{JobID: jobID})
}

type jobStatusResponse struct {
	JobID    string  `json:"job_id"`
	Status   string  `json:"status"`
	FilePath *string `json:"file_path,omitempty"`
	Error    *string `json:"error,omitempty"`
}

func (h *Handler) GetExport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	job, err := h.store.Get(r.Context(), id)
	if err != nil {
		log.Printf("[api] failed to fetch job %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to fetch job")
		return
	}
	if job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	writeJSON(w, http.StatusOK, jobStatusResponse{
		JobID:    job.ID,
		Status:   string(job.Status),
		FilePath: job.FilePath,
		Error:    job.Error,
	})
}

func (h *Handler) DownloadExport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	job, err := h.store.Get(r.Context(), id)
	if err != nil {
		log.Printf("[api] failed to fetch job %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to fetch job")
		return
	}
	if job == nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if job.Status != jobs.StatusDone || job.FilePath == nil {
		writeError(w, http.StatusConflict, "export is not ready yet")
		return
	}

	http.ServeFile(w, r, *job.FilePath)
}

// --- small helpers ---

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("[api] failed to encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
