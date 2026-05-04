// Package workers handles job execution and processing logic.
package workers

import (
	"context"
	"database/sql"
	"log"
	"testing"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/Arush71/jobqueue/internal/store"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func SetupTestDb() *db.Queries {
	database, err := sql.Open("postgres", "postgres://localhost:5432/job_queue_test?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	if err := database.Ping(); err != nil {
		log.Fatal(err)
	}
	dbQuery := db.New(database)
	return dbQuery
}

func createValidJob() *jobs.Job {
	return &jobs.Job{
		ImagePath: "output/test2.jpg",
		JobType:   "resize",
		Params: jobs.ParamsT{
			"width":  100,
			"height": 100,
		},
	}
}

func CreateTestJob(dbq *db.Queries, jobType jobs.JT, params jobs.ParamsT, imagePath string) (int64, error) {
	paramS, err := store.ToDBParams(params)
	if err != nil {
		return 0, err
	}
	id, err := dbq.CreateJob(context.Background(), db.CreateJobParams{
		Type:      db.JobType(jobType),
		ImagePath: imagePath,
		Params:    paramS,
		State:     db.JobStateQueued,
	})
	return id, err
}

func TestRetry(t *testing.T) {
	q := queue.SetupQueue()
	dbQuery := SetupTestDb()
	err := dbQuery.RemoveJobsFromDBForTest(context.Background())
	require.NoError(t, err)
	job := createValidJob()

	// Case 1: retry 0, expects, no error and counter to be 1 and state to be queueed.
	id, err := CreateTestJob(dbQuery, job.JobType, job.Params, job.ImagePath)
	require.NoError(t, err, "failed to create the job")
	job.JobId = id
	job.RetryCounter = 0
	err = manageRetries(dbQuery, q, *job)
	assert.NoError(t, err, "retry failed, unexpected behaviour for 0 retry counter")
	jobDB, err := dbQuery.GetJobById(context.Background(), id)
	require.NoError(t, err, "getting job failed, unexpected behaviour for 0 retry counter, ")
	assert.Equal(t, jobDB.State, db.JobStateQueued)
	assert.EqualValues(t, jobDB.RetryCounter, 1)
	idQ := q.GetWork()
	assert.Equal(t, idQ, id)

	// Case 2: retry more then the max, expects: no error, state changed to fail.

	id, err = CreateTestJob(dbQuery, job.JobType, job.Params, job.ImagePath)
	require.NoError(t, err, "failed to create the job")
	job.JobId = id
	job.RetryCounter = 6
	err = manageRetries(dbQuery, q, *job)
	assert.NoError(t, err, "retry failed, unexpected behaviour for 6 retry counter")
	jobDB, err = dbQuery.GetJobById(context.Background(), id)
	require.NoError(t, err, "getting job failed, unexpected behaviour for 6 retry counter")
	assert.Equal(t, jobDB.State, db.JobStateFail)
	done := make(chan int64, 1)
	go func() {
		done <- q.GetWork()
	}()
	select {
	case v := <-done:
		t.Fatalf("expected no job to be queued, but got %d", v)
	case <-time.After(100 * time.Millisecond):
	}
}
