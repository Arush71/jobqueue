package tests

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
	"github.com/Arush71/jobqueue/internal/workers"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/suite"
)

type JobQueueSuite struct {
	suite.Suite
	dbQ *db.Queries
	q   *queue.Queue
}

func (suite *JobQueueSuite) SetupSuite() {
	database, err := sql.Open("postgres", "postgres://localhost:5432/job_queue_test?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	if err := database.Ping(); err != nil {
		log.Fatal(err)
	}
	suite.dbQ = db.New(database)
}

func (suite *JobQueueSuite) SetupTest() {
	suite.ClearDb()
	suite.q = queue.SetupQueue()
	go workers.DoWork(suite.q, suite.dbQ)
	go workers.DoWork(suite.q, suite.dbQ)
}

func (suite *JobQueueSuite) createValidJob() *jobs.Job {
	return &jobs.Job{
		ImagePath: "output/test1.jpg",
		JobType:   "resize",
		Params: jobs.ParamsT{
			"width":  100,
			"height": 100,
		},
	}
}

func (suite *JobQueueSuite) ClearDb() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := suite.dbQ.RemoveJobsFromDBForTest(ctx); err != nil {
		suite.T().Fatalf("Failed to clear db %s", err.Error())
	}
}

func startTimeCtx(t time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), t)
	return ctx, cancel
}

func (suite *JobQueueSuite) CreateTestJob(jobType jobs.JT, params jobs.ParamsT, imagePath string, state db.JobState) (int64, error) {
	paramsT, err := store.ToDBParams(params)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	id, err := suite.dbQ.CreateJob(ctx, db.CreateJobParams{
		Type:      db.JobType(jobType),
		ImagePath: imagePath,
		Params:    paramsT,
		State:     state,
	})
	return id, err
}

func (suite *JobQueueSuite) WaitForExpectedState(jobID int64, expectedState db.JobState, unexpectedState db.JobState) db.Job {
	ctx, cancel := startTimeCtx(5 * time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			suite.T().Fatalf("Context exceeded, the worker has not yet finished the work.")
		default:
			ctx2, cancel2 := startTimeCtx(2 * time.Second)
			jobData, err := suite.dbQ.GetJobById(ctx2, jobID)
			cancel2()
			suite.Require().NoError(err)
			if jobData.State == expectedState {
				return jobData
			}
			if jobData.State == unexpectedState {
				suite.Require().FailNow("job failed, the job state turned to failure.")
			}
			time.Sleep(150 * time.Millisecond)
		}
	}
}

func TestJobQueueSuite(t *testing.T) {
	suite.Run(t, new(JobQueueSuite))
}
