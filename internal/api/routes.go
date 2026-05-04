package api

import "net/http"

// AddRoutes adds routes and the handler function to the mux
func AddRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("POST /jobs", h.CreateJob)
	// mux.HandleFunc("GET /jobs", )
	mux.HandleFunc("GET /jobs/{id}", h.GetJobsByID)
	// mux.HandleFunc("DELETE /jobs/{id}", )
}
