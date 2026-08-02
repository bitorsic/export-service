package api

import "net/http"

func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /exports", h.CreateExport)
	mux.HandleFunc("GET /exports/{id}", h.GetExport)
	mux.HandleFunc("GET /exports/{id}/download", h.DownloadExport)

	return mux
}
