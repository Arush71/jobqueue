// Entry point of the application
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

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

func setupDbAndEnv() (*pgxpool.Pool, *db.Queries) {
	if err := godotenv.Load(); err != nil {
		log.Println("Failed to load dotenv")
	}
	dbURL := os.Getenv("DB_URL")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	if err = pool.Ping(context.Background()); err != nil {
		log.Fatal("Unable to ping db:", err)
	}
	dbQuery := db.New(pool)
	return pool, dbQuery
}

func RestoreLostJobs(q *queue.Queue, dbQ *db.Queries) {
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	err := dbQ.UpdateJobStateAtRestart(ctx)
	if err != nil {
		log.Fatal("FATAL: failed to change state while recovering : " + err.Error())
		return
	}
	jobIDs, err := dbQ.GetLeftJobs(ctx)
	if err != nil {
		log.Fatal("FATAL: failed to recover jobs from db: " + err.Error())
		return
	}
	for _, v := range jobIDs {
		q.EnqueueJob(v)
	}
}

func main() {
	pool, dbQuery := setupDbAndEnv()
	defer pool.Close()
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
