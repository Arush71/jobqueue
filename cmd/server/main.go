package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/Arush71/jobqueue/internal/api"
	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/Arush71/jobqueue/internal/workers"
)

func setupHandler(dq *db.Queries) *api.Handler {
	Q := queue.SetupQueue()
	return &api.Handler{
		DbQ:   dq,
		Queue: Q,
	}
}

func setupDbAndEnv() *db.Queries {
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
	dbQuery := db.New(database)
	return dbQuery
}
func RestoreLostJobs(q *queue.Queue, dbQ *db.Queries) {
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	err := dbQ.UpdateJobStateAtRestart(ctx)
	if err != nil {
		log.Fatal("FATAL: failed to change state while recovering : " + err.Error())
		return
	}
	job_IDs, err := dbQ.GetLeftJobs(ctx)
	if err != nil {
		log.Fatal("FATAL: failed to recover jobs from db: " + err.Error())
		return
	}
	for _, v := range job_IDs {
		q.EnqueueJob(v)
	}
}
func main() {
	dbQuery := setupDbAndEnv()
	handler := setupHandler(dbQuery)
	RestoreLostJobs(handler.Queue, dbQuery)
	mux := http.NewServeMux()
	api.AddRoutes(mux, handler)
	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	for i := 0; i < 4; i++ {
		go workers.DoWork(handler.Queue, dbQuery)
	}
	fmt.Printf("Starting server...")
	log.Fatal(server.ListenAndServe())
}
