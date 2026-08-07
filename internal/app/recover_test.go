package app

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/Arush71/jobqueue/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecoverIntegration(t *testing.T) {
	_, queries := testutil.OpenTestDB(t)
	create := func(priority db.QueuePriority) int64 {
		id, err := queries.CreateJob(context.Background(), db.CreateJobParams{
			JobType: "flaky", State: db.JobStateQueued,
			Payload: []byte(`{"fail_rate":0,"delay_sec":0}`), JobPriority: priority,
		})
		require.NoError(t, err)
		return id
	}
	normalID := create(db.QueuePriorityNormal)
	highID := create(db.QueuePriorityHigh)
	_, err := queries.GetJobIfQueued(context.Background(), highID)
	require.NoError(t, err)
	successID := create(db.QueuePriorityLow)
	require.NoError(t, queries.SuccessJobWithResult(context.Background(), db.SuccessJobWithResultParams{ID: successID, Results: []byte(`{}`)}))
	failID := create(db.QueuePriorityLow)
	require.NoError(t, queries.UpdateJobId(context.Background(), db.UpdateJobIdParams{ID: failID, State: db.JobStateFail}))

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := queue.SetupQueue(logger, 1)
	a := &App{dbQ: queries, queue: q, logger: logger}
	require.NoError(t, a.Recover())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	got, err := q.GetWork(ctx)
	require.NoError(t, err)
	assert.Equal(t, highID, got)
	got, err = q.GetWork(ctx)
	require.NoError(t, err)
	assert.Equal(t, normalID, got)

	emptyCtx, emptyCancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer emptyCancel()
	_, err = q.GetWork(emptyCtx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
