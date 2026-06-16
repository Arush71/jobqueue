// Entry point of the application
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/Arush71/jobqueue/internal/api"
	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/Arush71/jobqueue/internal/workers"
)

func setupHandler(dq *db.Queries, log *slog.Logger, numWorkers int) *api.Handler {
	Q := queue.SetupQueue(log, numWorkers)
	return &api.Handler{
		DBQ:    dq,
		Queue:  Q,
		Logger: log,
	}
}

func setupDBAndEnv() (*pgxpool.Pool, *db.Queries, error) {
	if err := godotenv.Load(); err != nil {
		return nil, nil, fmt.Errorf("failed to load dotenv: %w", err)
	}
	dbURL := os.Getenv("DB_URL")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create a new db pool connection: %w", err)
	}
	if err = pool.Ping(context.Background()); err != nil {
		return nil, nil, fmt.Errorf("unable to ping the db pool: %w", err)
	}
	dbQuery := db.New(pool)
	return pool, dbQuery, nil
}

func RestoreLostJobs(q *queue.Queue, dbQ *db.Queries) error {
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	err := dbQ.UpdateJobStateAtRestart(ctx)
	if err != nil {
		return fmt.Errorf("failed to change job state at recovery in db: %w", err)
	}
	jobIDs, err := dbQ.GetLeftJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to recover jobs from db after changing the state: %w", err)
	}
	for _, v := range jobIDs {
		if err := q.EnqueueJob(v.ID, queue.Priority(v.JobPriority), true); err != nil {
			panic("got an unexpected error")
		}
	}
	return nil
}

func initilizeLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

func main() {
	logger := initilizeLogger()
	logger.Info("Logger initilized")
	pool, dbQuery, err := setupDBAndEnv()
	if err != nil {
		logger.Error("failed to setup db and/or env", "error", err)
		return
	}
	defer pool.Close()
	workersNum := runtime.NumCPU() * 2
	handler := setupHandler(dbQuery, logger, workersNum)
	schedular := queue.InitSchedular(handler.Queue)
	if err := RestoreLostJobs(handler.Queue, dbQuery); err != nil {
		logger.Error("failed to recover lost jobs", "error", err)
		return
	}
	workerPool := workers.NewPool(workersNum, logger, handler.Queue, schedular, dbQuery)
	workerPool.Start()
	mux := http.NewServeMux()
	api.AddRoutes(mux, handler)
	server := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	logger.Info("server starting...")
	if err := server.ListenAndServe(); err != nil {
		logger.Error("server crashed", "error", err)
	}
}
