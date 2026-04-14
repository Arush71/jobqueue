package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Arush71/jobqueue/internal/api"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/Arush71/jobqueue/internal/workers"
)

func setupHandler() *api.Handler {
	JobId := &jobs.JobId{
		Counter: 0,
	}
	Q := queue.SetupQueue()
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
	for i := 0; i < 4; i++ {
		go workers.DoWork(handler.Queue)
	}
	fmt.Printf("Starting server...")
	log.Fatal(server.ListenAndServe())
}
