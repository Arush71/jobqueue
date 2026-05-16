package tests

import (
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/Arush71/jobqueue/internal/workers"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
)

type JobQueueSuite struct {
	suite.Suite
	dbQ  *db.Queries
	q    *queue.Queue
	pool *pgxpool.Pool
}

func (suite *JobQueueSuite) SetupSuite() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://localhost:5432/job_queue_test?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	if err = pool.Ping(context.Background()); err != nil {
		log.Fatal("Unable to ping db:", err)
	}
	dbQuery := db.New(pool)
	suite.dbQ = dbQuery
	suite.pool = pool
}

func (suite *JobQueueSuite) SetupTest() {
	suite.ClearDb()
	suite.q = queue.SetupQueue()
	go workers.DoWork(suite.q, suite.dbQ)
	go workers.DoWork(suite.q, suite.dbQ)
}

func (suite *JobQueueSuite) TearDownSuite() {
	suite.pool.Close()
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

func (suite *JobQueueSuite) CreateTestJob(payload []byte, state db.JobState, jobType string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	id, err := suite.dbQ.CreateJob(ctx, db.CreateJobParams{
		State:   state,
		JobType: jobType,
		Payload: payload,
	})
	return id, err
}

func (suite *JobQueueSuite) WaitForExpectedState(jobID int64, expectedState db.JobState, unexpectedState db.JobState) db.Job {
	for {
		ctx, cancel := startTimeCtx(2 * time.Second)
		jobData, err := suite.dbQ.GetJobById(ctx, jobID)
		cancel()
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

func (suite *JobQueueSuite) CreateValidImageJob(imagePath string) db.Job {
	if imagePath == "" {
		imagePath = "output/test1.jpg"
	}
	type imagePayload struct {
		ImageJobType string             `json:"image_job_type"`
		ImagePath    string             `json:"image_path"`
		Params       map[string]float64 `json:"params"`
	}
	payload, err := json.Marshal(imagePayload{
		ImageJobType: "resize",
		ImagePath:    imagePath,
		Params: map[string]float64{
			"width":  100,
			"height": 200,
		},
	})
	if err != nil {
		suite.T().Fatal("Failed to parse image job payload to byte[]")
	}
	return db.Job{
		State:   db.JobStateQueued,
		JobType: jobs.JobImageType,
		Payload: payload,
	}
}

func (suite *JobQueueSuite) CreateValidFlakyJob(failRate float64, delay int) db.Job {
	type flakyPayload struct {
		FailRate float64 `json:"fail_rate"`
		DelaySec int     `json:"delay_sec"` // simulate work duration
	}
	payload, err := json.Marshal(flakyPayload{
		FailRate: failRate,
		DelaySec: delay,
	})
	if err != nil {
		suite.T().Fatal("Failed to parse flaky job payload to byte[]")
	}
	return db.Job{
		State:   db.JobStateQueued,
		JobType: jobs.JobFlakyType,
		Payload: payload,
	}
}

func (suite *JobQueueSuite) CreateInValidImageJob() db.Job {
	type imagePayload struct {
		ImageJobType string             `json:"image_job_type"`
		ImagePath    string             `json:"image_path"`
		Params       map[string]float64 `json:"params"`
	}
	payload, err := json.Marshal(imagePayload{
		ImageJobType: "resize",
		ImagePath:    " ",
		Params: map[string]float64{
			"width":   0.01,
			"height":  0.02,
			"aRandom": 34,
		},
	})
	if err != nil {
		suite.T().Fatal("Failed to parse image job payload to byte[]")
	}
	return db.Job{
		State:   db.JobStateQueued,
		JobType: jobs.JobImageType,
		Payload: payload,
	}
}

func TestJobQueueSuite(t *testing.T) {
	suite.Run(t, new(JobQueueSuite))
}
