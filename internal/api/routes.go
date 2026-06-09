package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// AddRoutes adds routes and the handler function to the mux
func AddRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("POST /jobs", h.CreateJob)
	// mux.HandleFunc("GET /jobs", )
	mux.HandleFunc("GET /jobs/{id}", h.GetJobsByID)
	mux.HandleFunc("GET /jobs/{id}/results", h.GetJobResult)

	mux.Handle("/metrics", promhttp.Handler())
}
