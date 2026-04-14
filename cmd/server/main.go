package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3/database"

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

func setupDbAndEnv() {
	if err := godotenv.Load(); err != nil {
		log.Println("Failed to load dotenv")
	}
	dbUrl := os.Getenv("DB_URL")
	database, err := sql.Open("postgres", dbUrl)
	if err != nil {
		log.Fatal(err)
	}
	if err := database.Ping(); err != nil {
		log.Fatal(err)
	}
	dbQuery := db
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
