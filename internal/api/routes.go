package api

import "net/http"

func AddRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("POST /jobs", h.CreateJob)
	// mux.HandleFunc("GET /jobs", )
	mux.HandleFunc("GET /jobs/{id}", h.GetJobsById)
	// mux.HandleFunc("DELETE /jobs/{id}", )
}
