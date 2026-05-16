package tests

import (
	"context"
	"log"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/queue"
)

func (suite *JobQueueSuite) TestANormalNonErrorWork() {
	// Case 1: A completely valid image job, with valid data.
	job := suite.CreateValidImageJob("")
	id, err := suite.CreateTestJob(job.Payload, job.State, job.JobType)
	suite.Require().NoError(err)
	suite.q.EnqueueJob(id)
	// Job queued
	jobData := suite.WaitForExpectedState(id, db.JobStateSuccess, db.JobStateFail)
	// worker should finish working on the job, and the state should be success.
	suite.Assert().EqualValues(0, jobData.RetryCounter)
	suite.Assert().False(jobData.Error.Valid)
	suite.Assert().NotNil(jobData.Results)
	suite.Assert().True(jobData.CompletedAt.Valid)

	// Case 2: valid flaky job.
	job = suite.CreateValidFlakyJob(0, 2)
	id, err = suite.CreateTestJob(job.Payload, job.State, job.JobType)
	suite.Require().NoError(err)
	suite.q.EnqueueJob(id)
	jobData = suite.WaitForExpectedState(id, db.JobStateSuccess, db.JobStateFail)
	suite.Assert().EqualValues(0, jobData.RetryCounter)
	suite.Assert().False(jobData.Error.Valid)
	suite.Assert().NotNil(jobData.Results)
	suite.Assert().True(jobData.CompletedAt.Valid)
}

func (suite *JobQueueSuite) TestInvalidData() {
	// Case 1: job image, wrong image path

	job := suite.CreateValidImageJob("lolPATH")
	id, err := suite.CreateTestJob(job.Payload, job.State, job.JobType)
	suite.Require().NoError(err)
	suite.q.EnqueueJob(id)
	// Job queued
	jobData := suite.WaitForExpectedState(id, db.JobStateFail, db.JobStateSuccess)
	// worker should finish working on the job, and the state should be fail.
	suite.Assert().EqualValues(jobs.MaxRetries, jobData.RetryCounter)
	suite.Assert().True(jobData.Error.Valid)
	suite.Assert().NotEmpty(jobData.Error.String)
	suite.Assert().Nil(jobData.Results)
	suite.Assert().True(jobData.CompletedAt.Valid)

	// Case 2: flaky handler failure rate 1

	job = suite.CreateValidFlakyJob(1, 2)
	id, err = suite.CreateTestJob(job.Payload, job.State, job.JobType)
	suite.Require().NoError(err)
	suite.q.EnqueueJob(id)
	// Job queued
	jobData = suite.WaitForExpectedState(id, db.JobStateFail, db.JobStateSuccess)
	// worker should finish working on the job, and the state should be fail.
	suite.Assert().EqualValues(jobs.MaxRetries, jobData.RetryCounter)
	suite.Assert().True(jobData.Error.Valid)
	suite.Assert().NotEmpty(jobData.Error.String)
	suite.Assert().Nil(jobData.Results)
	suite.Assert().True(jobData.CompletedAt.Valid)
}

func (suite *JobQueueSuite) TestInvalidJob() {
	// Should fail at the validation process with retry count 0.

	// Case 1: job image, wrong image job type.

	job := suite.CreateValidImageJob("")
	job.JobType = "random stuff"
	id, err := suite.CreateTestJob(job.Payload, job.State, job.JobType)
	suite.Require().NoError(err)
	suite.q.EnqueueJob(id)
	// Job queued
	jobData := suite.WaitForExpectedState(id, db.JobStateFail, db.JobStateSuccess)
	// worker should finish working on the job, and the state should be fail.
	suite.Assert().EqualValues(0, jobData.RetryCounter)
	suite.Assert().True(jobData.Error.Valid)
	suite.Assert().NotEmpty(jobData.Error.String)
	suite.Assert().Nil(jobData.Results)
	suite.Assert().True(jobData.CompletedAt.Valid)

	// Case 2: Invalid job params for image job.

	job = suite.CreateInValidImageJob()
	id, err = suite.CreateTestJob(job.Payload, job.State, job.JobType)
	suite.Require().NoError(err)
	suite.q.EnqueueJob(id)
	// Job queued
	jobData = suite.WaitForExpectedState(id, db.JobStateFail, db.JobStateSuccess)
	// worker should finish working on the job, and the state should be fail.

	suite.Assert().EqualValues(0, jobData.RetryCounter)
	suite.Assert().True(jobData.Error.Valid)
	suite.Assert().NotEmpty(jobData.Error.String)
	suite.Assert().Nil(jobData.Results)
	suite.Assert().True(jobData.CompletedAt.Valid)
}

func (suite *JobQueueSuite) TestJobTimeOut() {
	// Should fail with max retries and the failure should say timeout.

	job := suite.CreateValidFlakyJob(0.5, 31) // 31 seconds
	id, err := suite.CreateTestJob(job.Payload, job.State, job.JobType)
	suite.Require().NoError(err)
	suite.q.EnqueueJob(id)
	// Job queued
	jobData := suite.WaitForExpectedState(id, db.JobStateFail, db.JobStateSuccess)
	// worker should finish working on the job, and the state should be fail.
	suite.Assert().EqualValues(jobs.MaxRetries, jobData.RetryCounter)
	suite.Assert().True(jobData.Error.Valid)
	suite.Assert().Equal("job timeout", jobData.Error.String)
	suite.Assert().Nil(jobData.Results)
	suite.Assert().True(jobData.CompletedAt.Valid)
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

func (suite *JobQueueSuite) TestRestartRecovery() {
	// Case 1: Image job testing recovery
	job1 := suite.CreateValidImageJob("")
	job2 := suite.CreateValidImageJob("random image path lol")
	// job 2 should fail.
	job1ID, err := suite.CreateTestJob(job1.Payload, job1.State, job1.JobType)
	suite.Require().NoError(err)
	job2ID, err := suite.CreateTestJob(job2.Payload, job2.State, job2.JobType)
	suite.Require().NoError(err)
	RestoreLostJobs(suite.q, suite.dbQ)
	// now the jobs should be rescued, and queued back.
	jobData1 := suite.WaitForExpectedState(job1ID, db.JobStateSuccess, db.JobStateFail)
	jobData2 := suite.WaitForExpectedState(job2ID, db.JobStateFail, db.JobStateSuccess)
	suite.Assert().EqualValues(0, jobData1.RetryCounter)
	suite.Assert().EqualValues(jobs.MaxRetries, jobData2.RetryCounter)
	suite.Assert().True(jobData1.CompletedAt.Valid)
	suite.Assert().True(jobData2.CompletedAt.Valid)
}
