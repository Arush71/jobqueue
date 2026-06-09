package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/jobs/handler"
	"github.com/Arush71/jobqueue/internal/metrics"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ValidationFailedErr = errors.New("validation failed inside worker, invalid payload")
	FatalError          = errors.New("job panicked inside worker, fatal error")
)

// DoWork continuously pulls jobs from the queue and processes them.
func DoWork(q *queue.Queue, schedular *queue.Schedular, dbQ *db.Queries, log *slog.Logger, workerNum int) {
	for {
		jobID := q.GetWork()
		logger := log.With(slog.Int64("job_id", jobID), slog.Int("worker_num", workerNum))
		logger.Info("worker picked a job")
		job, err := getJobFromID(jobID, dbQ)
		if err != nil {
			logger.Error("worker failed to fetch job", "error", err)
			continue
		}
		logger = logger.With(slog.String("job_state", string(job.State)), slog.Int("retry_counter", int(job.RetryCounter)), slog.String("job_type", job.JobType))
		jobHandler, err := jobs.GetJobHandler(job.JobType)
		if err != nil {
			logger.Error("worker picked a job whose job handler doesn't exist", slog.String("job_type", job.JobType))
			if err := FailJob(jobID, dbQ, "Job type not supported", logger); err != nil {
				logger.Error("marking job as fail, failed", "error", err)
			}
			continue
		}
		start := time.Now()
		result, err := manageJobProcessing(job, jobHandler)
		duration := time.Since(start).Milliseconds()
		metrics.JobDuration.Observe(float64(duration))
		if err != nil {
			if errors.Is(err, ValidationFailedErr) || errors.Is(err, FatalError) {
				logger.Error(err.Error())
				if err := FailJob(jobID, dbQ, err.Error(), logger); err != nil {
					logger.Error("marking job as fail, failed", "error", err)
				}
				continue
			}
			logger.Debug("going to rety the job")
			if err := manageRetries(dbQ, q, job, err.Error(), logger, schedular, queue.Priority(job.JobPriority)); err != nil {
				logger.Error("failed to retry the job", "error", err)
			}
			continue
		}
		err = SuccessJob(job.ID, dbQ, result, job.JobType)
		if err != nil {
			logger.Error("failed to set the job to success", "error", err)
			continue
		}
		logger.Info("job successfull")
	}
}

func FailJob(jobID int64, dbQ *db.Queries, err string, loggger *slog.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errr := dbQ.FailJobWithError(ctx, db.FailJobWithErrorParams{
		ID: jobID,
		Error: pgtype.Text{
			Valid:  true,
			String: err,
		},
	})
	if errr != nil {
		metrics.JobPersistenceFailures.Inc()
		metrics.DBErrors.Inc()
		return errr
	}
	metrics.JobsFailed.Inc()
	loggger.Debug("job marked as failed")
	return nil
}

func getJobFromID(jobID int64, dbQ *db.Queries) (db.Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	job, err := dbQ.GetJobIfQueued(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.Job{}, fmt.Errorf("worker picked a id that's either not claimable or doesn't exsist: %w", err)
		}
		metrics.DBErrors.Inc()
		return db.Job{}, fmt.Errorf("db error, failed to fetch id: %w", err)
	}
	return job, nil
}

func SuccessJob(jobID int64, dbQ *db.Queries, result json.RawMessage, jobType string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := dbQ.SuccessJobWithResult(ctx, db.SuccessJobWithResultParams{
		ID:      jobID,
		Results: result,
	})
	if err != nil {
		metrics.JobPersistenceFailures.Inc()
		metrics.DBErrors.Inc()
		return err
	}
	metrics.JobsProcessed.Inc()
	metrics.JobsProcessedByType.WithLabelValues(jobType).Inc()
	return nil
}

func manageJobProcessing(job db.Job, handler handler.JobHandler) (res []byte, err error) {
	// validate job.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v: %w", r, FatalError)
		}
	}()
	err = handler.Validate(job.Payload)
	if err != nil {
		return nil, ValidationFailedErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err = handler.Process(ctx, job.Payload)
	return res, err
}

func manageRetries(dbQ *db.Queries, queue *queue.Queue, job db.Job, previousErr string, logger *slog.Logger, schdular *queue.Schedular, jobPriority queue.Priority) error {
	if job.RetryCounter >= jobs.MaxRetries {
		// Marks the end of the job.
		if err := FailJob(job.ID, dbQ, previousErr, logger); err != nil {
			return fmt.Errorf("failed to set the job to fail in manageRetries, after retry counter exceeded the max: %w", err)
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := dbQ.UpdateRetryCounterAndChangeState(ctx, job.ID)
	if err != nil {
		metrics.DBErrors.Inc()
		return fmt.Errorf("failed to change job state to queue and increment retry_counter in retry: %w", err)
	}
	base := 10
	half := (base * (1 << job.RetryCounter)) / 2
	waitTime := time.Now().Add(time.Duration(half+rand.IntN(half)) * time.Second)
	schdular.ScheduleJob(job.ID, waitTime, jobPriority)
	metrics.JobsRetry.Inc()
	return nil
}
