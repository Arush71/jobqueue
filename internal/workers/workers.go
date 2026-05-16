package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/jobs/handler"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/jackc/pgx/v5/pgtype"
)

var ValidationFailedErr = errors.New("validation failed, invalid payload")

// DoWork continuously pulls jobs from the queue and processes them.
func DoWork(q *queue.Queue, dbQ *db.Queries) {
	for {
		jobID := q.GetWork()
		log.Println("worker picked job: id =", jobID)
		job, err := getJobFromID(jobID, dbQ)
		if err != nil {
			continue
		}
		jobHandler, err := jobs.GetJobHandler(job.JobType)
		if err != nil {
			log.Printf("Error from worker: picked a job whose job handler does not exist")
			if err := FailJob(jobID, dbQ, "Job type not supported"); err != nil {
				log.Printf("[CRITICAL] invariant violation: Job state change to fail, failed for id:%d", jobID)
			}
			continue
		}
		result, err := manageJobProcessing(job, jobHandler)
		if err != nil {
			if errors.Is(err, ValidationFailedErr) {
				if err := FailJob(jobID, dbQ, ValidationFailedErr.Error()); err != nil {
					log.Printf("[CRITICAL] invariant violation: Job state change to fail, failed for id:%d , err: %s", jobID, err.Error())
				}
				continue
			}
			manageRetries(dbQ, q, job, err.Error())
			continue
		}
		err = SuccessJob(job.ID, dbQ, result)
		if err != nil {
			continue
		}
		log.Println("Job successfull", job.ID)
	}
}

func FailJob(jobID int64, dbQ *db.Queries, err string) error {
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
		log.Printf("Important [CRITICAL]: Job state to fail, failed for id %d , error: %s", jobID, errr.Error())
		return errr
	}
	return nil
}

func getJobFromID(jobID int64, dbQ *db.Queries) (db.Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	job, err := dbQ.GetJobIfQueued(ctx, jobID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("Important: worker picked id that's not available in db or is already taken id:%d", jobID)
			return db.Job{}, fmt.Errorf("%d not claimable by the worker", jobID)
		}
		log.Printf("Important: db error, fail to fetch job with id: %d", jobID)
		return db.Job{}, fmt.Errorf("failed to fetch job of the id %d", jobID)
	}
	return job, nil
}

func SuccessJob(jobID int64, dbQ *db.Queries, result json.RawMessage) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := dbQ.SuccessJobWithResult(ctx, db.SuccessJobWithResultParams{
		ID:      jobID,
		Results: result,
	})
	if err != nil {
		log.Printf("Important [CRITICAL]: Job state to success failed for id %d , error: %s", jobID, err.Error())
		return err
	}
	return nil
}

func manageJobProcessing(job db.Job, handler handler.JobHandler) ([]byte, error) {
	// validate job.
	err := handler.Validate(job.Payload)
	if err != nil {
		return nil, ValidationFailedErr
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := handler.Process(ctx, job.Payload)
	return result, err
}

func manageRetries(dbQ *db.Queries, queue *queue.Queue, job db.Job, previousErr string) error {
	if job.RetryCounter >= jobs.MaxRetries {
		// Marks the end of the job.
		err := FailJob(job.ID, dbQ, previousErr)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := dbQ.UpdateRetryCounterAndChangeState(ctx, job.ID)
	if err != nil {
		log.Printf("Important [CRITICAL]: failed to change job state to queue and increment retry_counte , error: %s , for id: %d", err.Error(), job.ID)
		return err
	}
	queue.EnqueueJob(job.ID)
	return nil
}
