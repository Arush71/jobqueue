package workers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/jobs"
	jobhandler "github.com/Arush71/jobqueue/internal/jobs/handler"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func workerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newWorkerMock(t *testing.T) (*db.Queries, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	return db.New(mock), mock
}

type fakeHandler struct {
	validateErr error
	process     func(context.Context, []byte) ([]byte, error)
}

func (h fakeHandler) Validate([]byte) error { return h.validateErr }
func (h fakeHandler) Process(ctx context.Context, payload []byte) ([]byte, error) {
	return h.process(ctx, payload)
}

var _ jobhandler.JobHandler = fakeHandler{}

func TestManageJobProcessing(t *testing.T) {
	job := db.Job{Payload: []byte(`{"x":1}`)}
	result, err := manageJobProcessing(context.Background(), job, fakeHandler{process: func(_ context.Context, payload []byte) ([]byte, error) {
		assert.JSONEq(t, `{"x":1}`, string(payload))
		return []byte(`{"ok":true}`), nil
	}}, time.Second)
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(result))

	_, err = manageJobProcessing(context.Background(), job, fakeHandler{validateErr: errors.New("bad"), process: func(context.Context, []byte) ([]byte, error) {
		t.Fatal("Process called after validation failure")
		return nil, nil
	}}, time.Second)
	assert.ErrorIs(t, err, ErrValidationFailed)

	_, err = manageJobProcessing(context.Background(), job, fakeHandler{process: func(context.Context, []byte) ([]byte, error) {
		panic("boom")
	}}, time.Second)
	assert.ErrorIs(t, err, ErrFatalJob)
}

func TestManageJobProcessingTimeoutAndParentCancellation(t *testing.T) {
	blocking := fakeHandler{process: func(ctx context.Context, _ []byte) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	_, err := manageJobProcessing(context.Background(), db.Job{}, blocking, 10*time.Millisecond)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = manageJobProcessing(ctx, db.Job{}, blocking, time.Second)
	assert.ErrorIs(t, err, context.Canceled)
}

type scheduledCall struct {
	id       int64
	at       time.Time
	priority queue.Priority
}

type recordingScheduler struct{ calls chan scheduledCall }

func (s recordingScheduler) ScheduleJob(id int64, at time.Time, priority queue.Priority) {
	s.calls <- scheduledCall{id: id, at: at, priority: priority}
}

func TestRetryDelayBounds(t *testing.T) {
	for retry := int16(0); retry <= jobs.MaxRetries; retry++ {
		half := time.Duration((10*(1<<retry))/2) * time.Second
		for range 25 {
			d := retryDelay(retry)
			assert.GreaterOrEqual(t, d, half)
			assert.Less(t, d, 2*half)
		}
	}
}

func TestManageRetriesSchedulesAndPersists(t *testing.T) {
	queries, mock := newWorkerMock(t)
	mock.ExpectExec("UPDATE jobs").WithArgs(int64(9)).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	scheduler := recordingScheduler{calls: make(chan scheduledCall, 1)}
	before := time.Now()
	err := manageRetries(queries, db.Job{ID: 9, RetryCounter: 0}, "temporary", workerLogger(), scheduler, queue.High)
	require.NoError(t, err)
	call := <-scheduler.calls
	assert.Equal(t, int64(9), call.id)
	assert.Equal(t, queue.High, call.priority)
	assert.False(t, call.at.Before(before.Add(5*time.Second)))
	assert.True(t, call.at.Before(before.Add(10*time.Second)))
}

func TestManageRetriesFailsAtMaximum(t *testing.T) {
	queries, mock := newWorkerMock(t)
	mock.ExpectExec("UPDATE jobs").WithArgs(int64(9), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	scheduler := recordingScheduler{calls: make(chan scheduledCall, 1)}
	err := manageRetries(queries, db.Job{ID: 9, RetryCounter: jobs.MaxRetries}, "permanent", workerLogger(), scheduler, queue.Normal)
	require.NoError(t, err)
	select {
	case <-scheduler.calls:
		t.Fatal("max-retry job was scheduled again")
	default:
	}
}

func TestPersistenceHelpers(t *testing.T) {
	queries, mock := newWorkerMock(t)
	mock.ExpectExec("UPDATE jobs").WithArgs(int64(1), []byte(`{"ok":true}`)).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, SuccessJob(1, queries, json.RawMessage(`{"ok":true}`), "flaky"))
	mock.ExpectExec("UPDATE jobs").WithArgs(int64(2), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	require.NoError(t, FailJob(2, queries, "broken", workerLogger()))
}

func TestPersistenceHelpersReturnDBErrors(t *testing.T) {
	queries, mock := newWorkerMock(t)
	mock.ExpectExec("UPDATE jobs").WithArgs(int64(1), pgxmock.AnyArg()).WillReturnError(errors.New("db error"))
	assert.Error(t, SuccessJob(1, queries, nil, "flaky"))
	mock.ExpectExec("UPDATE jobs").WithArgs(int64(2), pgxmock.AnyArg()).WillReturnError(errors.New("db error"))
	assert.Error(t, FailJob(2, queries, "broken", workerLogger()))
}

func queuedJobRows(id int64, jobType string, payload []byte, retry int16) *pgxmock.Rows {
	now := time.Now()
	return pgxmock.NewRows([]string{"id", "state", "created_at", "updated_at", "retry_counter", "job_type", "payload", "results", "error", "completed_at", "job_priority"}).
		AddRow(id, "processing", now, now, retry, jobType, payload, nil, nil, nil, "normal")
}

func TestGetJobFromID(t *testing.T) {
	queries, mock := newWorkerMock(t)
	mock.ExpectQuery("UPDATE jobs").WithArgs(int64(3)).WillReturnRows(queuedJobRows(3, "flaky", []byte(`{}`), 0))
	job, err := getJobFromID(3, queries)
	require.NoError(t, err)
	assert.Equal(t, int64(3), job.ID)

	mock.ExpectQuery("UPDATE jobs").WithArgs(int64(4)).WillReturnError(pgx.ErrNoRows)
	_, err = getJobFromID(4, queries)
	assert.Error(t, err)
}

func TestDoWorkSuccessfulJob(t *testing.T) {
	queries, mock := newWorkerMock(t)
	payload := []byte(`{"fail_rate":0,"delay_sec":0}`)
	mock.ExpectQuery("UPDATE jobs").WithArgs(int64(7)).WillReturnRows(queuedJobRows(7, jobs.JobFlakyType, payload, 0))
	mock.ExpectExec("UPDATE jobs").WithArgs(int64(7), []byte(`{"status":"success"}`)).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	q := queue.SetupQueue(workerLogger(), 1)
	require.NoError(t, q.EnqueueJob(7, queue.Normal, false))
	s := queue.InitScheduler(q)
	t.Cleanup(s.Stop)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		DoWork(q, s, queries, workerLogger(), 1, ctx)
		close(done)
	}()
	require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}
}

func TestDoWorkTerminalFailures(t *testing.T) {
	for _, tc := range []struct {
		name    string
		jobType string
		payload []byte
	}{
		{"unsupported handler", "missing", []byte(`{}`)},
		{"invalid payload", jobs.JobFlakyType, []byte(`{"fail_rate":2,"delay_sec":0}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			queries, mock := newWorkerMock(t)
			mock.ExpectQuery("UPDATE jobs").WithArgs(int64(8)).WillReturnRows(queuedJobRows(8, tc.jobType, tc.payload, 0))
			mock.ExpectExec("UPDATE jobs").WithArgs(int64(8), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			q := queue.SetupQueue(workerLogger(), 1)
			require.NoError(t, q.EnqueueJob(8, queue.Normal, false))
			s := queue.InitScheduler(q)
			t.Cleanup(s.Stop)
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				DoWork(q, s, queries, workerLogger(), 1, ctx)
				close(done)
			}()
			require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, time.Second, 10*time.Millisecond)
			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("worker did not stop")
			}
		})
	}
}

func TestWorkerPoolStartStop(t *testing.T) {
	queries, _ := newWorkerMock(t)
	q := queue.SetupQueue(workerLogger(), 2)
	s := queue.InitScheduler(q)
	t.Cleanup(s.Stop)
	p := NewPool(2, workerLogger(), q, s, queries)
	p.Start()
	p.Stop(time.Second)
}
