// Package app orchastrates everything
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"

	"github.com/Arush71/jobqueue/internal/api"
	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/Arush71/jobqueue/internal/server"
	"github.com/Arush71/jobqueue/internal/workers"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// App info
type App struct {
	dbPool     *pgxpool.Pool
	dbQ        *db.Queries
	workerPool *workers.Pool
	queue      *queue.Queue
	schedular  *queue.Schedular
	logger     *slog.Logger
	server     *server.Server
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

// StartApp inits the app
func StartApp() (*App, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	logger.Info("Logger initilized")
	pool, dbQ, err := setupDBAndEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to setup db and/or env: %w", err)
	}
	workersNum := runtime.NumCPU() * 2
	q := queue.SetupQueue(logger, workersNum)
	handler := &api.Handler{
		Logger: logger,
		Queue:  q,
		DBQ:    dbQ,
	}
	schedular := queue.InitSchedular(q)
	workerPool := workers.NewPool(workersNum, logger, q, schedular, dbQ)
	workerPool.Start()
	mux := http.NewServeMux()
	api.AddRoutes(mux, handler)
	server := server.New(logger, mux)
	go func() {
		if err := server.Run(); err != nil {
			logger.Error("server crashed", "error", err)
			panic("server crashed")
		}
	}()
	return &App{
		dbPool:     pool,
		dbQ:        dbQ,
		workerPool: workerPool,
		queue:      q,
		schedular:  schedular,
		server:     server,
		logger:     logger,
	}, nil
}
