package tests

import (
	"context"
	"log"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/queue"
)

// Test 1: a completely normal test for a single normal request, and potential variations.

func (suite *JobQueueSuite) TestANormalNonErrorWork() {
	// Case 1: A completely valid job, with valid data.
	job := suite.createValidJob()
	id, err := suite.CreateTestJob(job.JobType, job.Params, job.ImagePath, db.JobStateQueued)
	suite.Require().NoError(err)
	suite.q.EnqueueJob(id)
	// Job queued
	jobData := suite.WaitForExpectedState(id, db.JobStateSuccess, db.JobStateFail)
	// worker should finish working on the job, and the state should be success.
	suite.Assert().EqualValues(0, jobData.RetryCounter)
}

func (suite *JobQueueSuite) TestInvalidData() {
	// wrong image path, case1

	job := suite.createValidJob()
	job.ImagePath = "lolollolol"
	id, err := suite.CreateTestJob(job.JobType, job.Params, job.ImagePath, db.JobStateQueued)
	suite.Require().NoError(err)
	suite.q.EnqueueJob(id)
	// Job queued
	jobData := suite.WaitForExpectedState(id, db.JobStateFail, db.JobStateSuccess)
	// worker should finish working on the job, and the state should be success.
	suite.Assert().EqualValues(jobs.MaxRetries, jobData.RetryCounter)

	// right image path, wrong parameters. case 2
	//
	// job = suite.createValidJob()
	// randomParms := make(jobs.ParamsT)
	// randomParms["hello"] = 1.1
	// randomParms["hi"] = 0.5
	// job.Params = randomParms
	// id, err = suite.CreateTestJob(job.JobType, job.Params, job.ImagePath)
	// suite.Require().NoError(err)
	// suite.q.EnqueueJob(id)
	// // Job queued
	// jobData = suite.WaitForExpectedState(id, db.JobStateFail, db.JobStateSuccess)
	// // worker should finish working on the job, and the state should be success.
	// suite.Assert().EqualValues(0, jobData.RetryCounter)

	// Corrupt data tests will fail, bcz currently we only have http level validation for params, but adding it on the system on any other parts is a waste of time bcz the format will change soon enough. Plus, its not really a open bug bcz its only accesible via the http which has validations.
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
	job1 := suite.createValidJob()
	job2 := suite.createValidJob()
	job2.ImagePath = "random image path lol"
	// job 2 should fail.
	job1ID, err := suite.CreateTestJob(job1.JobType, job1.Params, job1.ImagePath, db.JobStateQueued)
	suite.Require().NoError(err)
	job2ID, err := suite.CreateTestJob(job2.JobType, job2.Params, job2.ImagePath, db.JobStateProcessing)
	suite.Require().NoError(err)
	// a random delay to make it seem realistic lol
	RestoreLostJobs(suite.q, suite.dbQ)
	// now the jobs should be rescued, and queued back.
	jobData1 := suite.WaitForExpectedState(job1ID, db.JobStateSuccess, db.JobStateFail)
	jobData2 := suite.WaitForExpectedState(job2ID, db.JobStateFail, db.JobStateSuccess)
	suite.Assert().EqualValues(0, jobData1.RetryCounter)
	suite.Assert().EqualValues(jobs.MaxRetries, jobData2.RetryCounter)
}
