package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Arush71/jobqueue/internal/api"
	"github.com/Arush71/jobqueue/internal/types"
)

func setupHandler() *api.Handler {
	JobId := &types.JobId{
		Counter: 0,
	}
	Q := types.SetupQ()
	return &api.Handler{
		JobId: JobId,
		Queue: Q,
	}
}

func main() {
	handler := setupHandler()
	mux := http.NewServeMux()
	api.AddRoutes(mux, handler)
	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	fmt.Printf("Starting server...")
	log.Fatal(server.ListenAndServe())
}
