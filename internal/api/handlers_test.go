package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newHandlerTest(t *testing.T) (*Handler, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Handler{DBQ: db.New(mock), Queue: queue.SetupQueue(logger, 2), Logger: logger}, mock
}

func performRequest(h *Handler, method, target, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	AddRoutes(mux, h)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(method, target, strings.NewReader(body)))
	return recorder
}

func TestCreateJob(t *testing.T) {
	h, mock := newHandlerTest(t)
	payload := `{"fail_rate":0,"delay_sec":0}`
	mock.ExpectQuery("INSERT INTO jobs").
		WithArgs("flaky", db.JobStateQueued, []byte(payload), db.QueuePriorityNormal).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(12)))

	r := performRequest(h, http.MethodPost, "/jobs", `{"job_type":"flaky","payload":`+payload+`}`)
	assert.Equal(t, http.StatusCreated, r.Code)
	assert.JSONEq(t, `{"job_id":12}`, r.Body.String())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	id, err := h.Queue.GetWork(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(12), id)
}

func TestCreateJobWithExplicitPriority(t *testing.T) {
	h, mock := newHandlerTest(t)
	payload := `{"fail_rate":0,"delay_sec":0}`
	mock.ExpectQuery("INSERT INTO jobs").
		WithArgs("flaky", db.JobStateQueued, []byte(payload), db.QueuePriorityHigh).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(2)))
	r := performRequest(h, http.MethodPost, "/jobs", `{"job_type":"flaky","priority":"high","payload":`+payload+`}`)
	assert.Equal(t, http.StatusCreated, r.Code)
}

func TestCreateJobValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		code int
	}{
		{"malformed", `{`, http.StatusBadRequest},
		{"unknown field", `{"job_type":"flaky","payload":{},"extra":1}`, http.StatusBadRequest},
		{"unknown job", `{"job_type":"missing","payload":{}}`, http.StatusNotFound},
		{"invalid payload", `{"job_type":"flaky","payload":{"fail_rate":2,"delay_sec":0}}`, http.StatusBadRequest},
		{"invalid priority", `{"job_type":"flaky","priority":"urgent","payload":{"fail_rate":0,"delay_sec":0}}`, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newHandlerTest(t)
			r := performRequest(h, http.MethodPost, "/jobs", tc.body)
			assert.Equal(t, tc.code, r.Code)
			assert.Equal(t, "application/json", r.Header().Get("Content-Type"))
		})
	}
}

func TestCreateJobRejectsOversizedBody(t *testing.T) {
	h, _ := newHandlerTest(t)
	body := `{"job_type":"flaky","payload":{"fail_rate":0,"delay_sec":0},"padding":"` +
		strings.Repeat("x", maxRequestBodyBytes) + `"}`
	r := performRequest(h, http.MethodPost, "/jobs", body)
	assert.Equal(t, http.StatusBadRequest, r.Code)
}

func TestCreateJobFullQueueAndDBError(t *testing.T) {
	h, _ := newHandlerTest(t)
	for i := range 64 {
		require.NoError(t, h.Queue.EnqueueJob(int64(i), queue.Normal, false))
	}
	r := performRequest(h, http.MethodPost, "/jobs", `{"job_type":"flaky","payload":{"fail_rate":0,"delay_sec":0}}`)
	assert.Equal(t, http.StatusServiceUnavailable, r.Code)

	h, mock := newHandlerTest(t)
	payload := `{"fail_rate":0,"delay_sec":0}`
	mock.ExpectQuery("INSERT INTO jobs").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(errors.New("db unavailable"))
	r = performRequest(h, http.MethodPost, "/jobs", `{"job_type":"flaky","payload":`+payload+`}`)
	assert.Equal(t, http.StatusInternalServerError, r.Code)
}

func jobRows() *pgxmock.Rows {
	now := time.Now()
	return pgxmock.NewRows([]string{"id", "state", "created_at", "updated_at", "retry_counter", "job_type", "payload", "results", "error", "completed_at", "job_priority"}).
		AddRow(int64(4), "queued", now, now, int16(1), "flaky", []byte(`{"x":1}`), nil, nil, nil, "normal")
}

func TestGetJobByID(t *testing.T) {
	h, mock := newHandlerTest(t)
	mock.ExpectQuery("SELECT (.+) FROM jobs WHERE id").WithArgs(int64(4)).WillReturnRows(jobRows())
	r := performRequest(h, http.MethodGet, "/jobs/4", "")
	assert.Equal(t, http.StatusOK, r.Code)
	assert.Contains(t, r.Body.String(), `"job_state":"queued"`)

	r = performRequest(h, http.MethodGet, "/jobs/nope", "")
	assert.Equal(t, http.StatusBadRequest, r.Code)
}

func TestGetJobByIDNotFoundAndDBError(t *testing.T) {
	h, mock := newHandlerTest(t)
	mock.ExpectQuery("SELECT (.+) FROM jobs WHERE id").WithArgs(int64(7)).WillReturnError(pgx.ErrNoRows)
	r := performRequest(h, http.MethodGet, "/jobs/7", "")
	assert.Equal(t, http.StatusNotFound, r.Code)

	h, mock = newHandlerTest(t)
	mock.ExpectQuery("SELECT (.+) FROM jobs WHERE id").WithArgs(int64(8)).WillReturnError(errors.New("db error"))
	r = performRequest(h, http.MethodGet, "/jobs/8", "")
	assert.Equal(t, http.StatusInternalServerError, r.Code)
}

func TestGetJobResultsByState(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		state   string
		results any
		errText any
		done    any
		code    int
	}{
		{"queued", "queued", nil, nil, nil, http.StatusAccepted},
		{"processing", "processing", nil, nil, nil, http.StatusAccepted},
		{"success", "success", []byte(`{"ok":true}`), nil, now, http.StatusOK},
		{"fail", "fail", nil, "broken", now, http.StatusGone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, mock := newHandlerTest(t)
			rows := pgxmock.NewRows([]string{"state", "results", "error", "completed_at"}).AddRow(tc.state, tc.results, tc.errText, tc.done)
			mock.ExpectQuery("SELECT state, results, error,completed_at FROM jobs").WithArgs(int64(3)).WillReturnRows(rows)
			r := performRequest(h, http.MethodGet, "/jobs/3/results", "")
			assert.Equal(t, tc.code, r.Code)
			assert.Contains(t, r.Body.String(), `"state":"`+tc.state+`"`)
		})
	}
}

func TestGetJobResultsErrorsAndRouteMethods(t *testing.T) {
	h, mock := newHandlerTest(t)
	mock.ExpectQuery("SELECT state, results, error,completed_at FROM jobs").WithArgs(int64(5)).WillReturnError(pgx.ErrNoRows)
	r := performRequest(h, http.MethodGet, "/jobs/5/results", "")
	assert.Equal(t, http.StatusNotFound, r.Code)

	h, mock = newHandlerTest(t)
	mock.ExpectQuery("SELECT state, results, error,completed_at FROM jobs").WithArgs(int64(6)).WillReturnError(errors.New("db error"))
	r = performRequest(h, http.MethodGet, "/jobs/6/results", "")
	assert.Equal(t, http.StatusInternalServerError, r.Code)

	r = performRequest(h, http.MethodGet, "/jobs/not-an-id/results", "")
	assert.Equal(t, http.StatusBadRequest, r.Code)
	r = performRequest(h, http.MethodPut, "/jobs", "")
	assert.Equal(t, http.StatusMethodNotAllowed, r.Code)
}
