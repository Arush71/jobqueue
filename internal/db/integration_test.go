package db_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/testutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createJob(t *testing.T, q *db.Queries, priority db.QueuePriority) int64 {
	t.Helper()
	id, err := q.CreateJob(context.Background(), db.CreateJobParams{
		JobType: "flaky", State: db.JobStateQueued,
		Payload: []byte(`{"fail_rate":0,"delay_sec":0}`), JobPriority: priority,
	})
	require.NoError(t, err)
	return id
}

func TestJobStateTransitionsIntegration(t *testing.T) {
	_, q := testutil.OpenTestDB(t)
	id := createJob(t, q, db.QueuePriorityNormal)

	claimed, err := q.GetJobIfQueued(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, db.JobStateProcessing, claimed.State)
	_, err = q.GetJobIfQueued(context.Background(), id)
	assert.ErrorIs(t, err, pgx.ErrNoRows)

	require.NoError(t, q.UpdateRetryCounterAndChangeState(context.Background(), id))
	retried, err := q.GetJobById(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, db.JobStateQueued, retried.State)
	assert.EqualValues(t, 1, retried.RetryCounter)

	require.NoError(t, q.SuccessJobWithResult(context.Background(), db.SuccessJobWithResultParams{ID: id, Results: []byte(`{"ok":true}`)}))
	result, err := q.GetResultFromJob(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, db.JobStateSuccess, result.State)
	assert.JSONEq(t, `{"ok":true}`, string(result.Results))
	assert.True(t, result.CompletedAt.Valid)

	failedID := createJob(t, q, db.QueuePriorityLow)
	require.NoError(t, q.FailJobWithError(context.Background(), db.FailJobWithErrorParams{ID: failedID, Error: pgtype.Text{String: "broken", Valid: true}}))
	failed, err := q.GetResultFromJob(context.Background(), failedID)
	require.NoError(t, err)
	assert.Equal(t, db.JobStateFail, failed.State)
	assert.Equal(t, "broken", failed.Error.String)
	assert.True(t, failed.CompletedAt.Valid)
}

func TestAtomicClaimIntegration(t *testing.T) {
	_, q := testutil.OpenTestDB(t)
	id := createJob(t, q, db.QueuePriorityHigh)

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			_, err := q.GetJobIfQueued(context.Background(), id)
			results <- err
		})
	}
	wg.Wait()
	close(results)
	var successes, noRows int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case assert.ErrorIs(t, err, pgx.ErrNoRows):
			noRows++
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, noRows)
}

func TestRecoveryQueriesIntegration(t *testing.T) {
	_, q := testutil.OpenTestDB(t)
	queued := createJob(t, q, db.QueuePriorityNormal)
	processing := createJob(t, q, db.QueuePriorityHigh)
	_, err := q.GetJobIfQueued(context.Background(), processing)
	require.NoError(t, err)
	success := createJob(t, q, db.QueuePriorityLow)
	require.NoError(t, q.SuccessJobWithResult(context.Background(), db.SuccessJobWithResultParams{ID: success, Results: []byte(`{}`)}))

	require.NoError(t, q.UpdateJobStateAtRestart(context.Background()))
	left, err := q.GetLeftJobs(context.Background())
	require.NoError(t, err)
	require.Len(t, left, 2)
	assert.Equal(t, []int64{queued, processing}, []int64{left[0].ID, left[1].ID})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	job, err := q.GetJobById(ctx, success)
	require.NoError(t, err)
	assert.Equal(t, db.JobStateSuccess, job.State)
}
