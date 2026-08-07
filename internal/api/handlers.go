// Package api provides HTTP handlers for interacting with the job queue system.
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/helpers"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/metrics"
	"github.com/Arush71/jobqueue/internal/queue"
)

const maxRequestBodyBytes = 1 << 20

// Handler holds dependencies required for handling HTTP requests.
type Handler struct {
	DBQ    *db.Queries
	Queue  *queue.Queue
	Logger *slog.Logger
}

// CreateJob handles job creation requests, validates input,
// persists the job, and enqueues it for processing.
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	var req jobs.JobHandlerTypeHTTP
	if err := helpers.ReadJson(r, &req, h.Logger); err != nil {
		helpers.BadRequestError(w)
		return
	}
	jobHandler, err := jobs.GetJobHandler(req.JobType)
	if err != nil {
		helpers.Error(w, http.StatusNotFound, err.Error())
		return
	}
	if err := jobHandler.Validate(req.Payload); err != nil {
		helpers.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}
	if err := h.Queue.CheckCapacity(queue.Priority(req.Priority)); err != nil {
		if errors.Is(err, queue.ErrQueueLimitReached) {
			helpers.Error(w, http.StatusServiceUnavailable, "queue is full, try again later.")
		} else {
			helpers.BadRequestError(w)
		}
		return
	}
	jobID, err := h.DBQ.CreateJob(r.Context(), db.CreateJobParams{
		JobType:     req.JobType,
		State:       db.JobStateQueued,
		Payload:     req.Payload,
		JobPriority: db.QueuePriority(req.Priority),
	})
	if err != nil {
		metrics.DBErrors.Inc()
		h.Logger.Error("failed to create a job", slog.Int64("job_id", jobID), "error", err)
		helpers.InternalServerError(w)
		return
	}
	if err := h.Queue.EnqueueJob(jobID, queue.Priority(req.Priority), true); err != nil {
		panic("should not be possible: priority should be valid with no limit err")
	}
	h.Logger.Info("job created and enqueued", slog.Int64("job_id", jobID))
	type res struct {
		ID int64 `json:"job_id"`
	}
	helpers.WriteJson(w, http.StatusCreated, res{
		ID: jobID,
	})
}

// GetJobsByID handles fetching a job by its ID and returns its current state.
func (h *Handler) GetJobsByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		helpers.Error(w, http.StatusBadRequest, "invalid job id: "+idStr)
		return
	}
	job, err := h.DBQ.GetJobById(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.NotFoundError(w)
			return
		}
		metrics.DBErrors.Inc()
		h.Logger.Error("db error, failed to fetch job from handler", slog.Int64("job_id", id), "error", err)
		helpers.InternalServerError(w)
		return
	}
	type JobRes struct {
		JobID        int64           `json:"job_id"`
		JobType      string          `json:"job_type"`
		State        string          `json:"job_state"`
		Payload      json.RawMessage `json:"params"`
		RetryCounter int16           `json:"retry_counter"`
	}
	helpers.WriteJson(w, http.StatusOK, JobRes{
		JobID:        job.ID,
		JobType:      job.JobType,
		State:        string(job.State),
		Payload:      job.Payload,
		RetryCounter: job.RetryCounter,
	})
}

// GetJobResult returns the result of a job, 404 if it has failed or is not ready.
func (h *Handler) GetJobResult(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		helpers.Error(w, http.StatusBadRequest, "invalid job id: "+idStr)
		return
	}
	data, err := h.DBQ.GetResultFromJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.NotFoundError(w)
			return
		}
		metrics.DBErrors.Inc()
		h.Logger.Error("db error, failed to get job results", slog.Int64("job_id", id), "error", err)
		helpers.InternalServerError(w)
		return
	}
	type base struct {
		Results     []byte    `json:"results,omitempty"`
		Error       string    `json:"error,omitempty"`
		State       string    `json:"state"`
		CompletedAt time.Time `json:"completed_at"`
	}
	switch data.State {
	case db.JobStateFail:
		helpers.WriteJson(w, http.StatusGone, base{
			CompletedAt: data.CompletedAt.Time,
			State:       string(data.State),
			Error:       data.Error.String,
		})
		return
	case db.JobStateProcessing, db.JobStateQueued:
		helpers.WriteJson(w, http.StatusAccepted, base{
			State: string(data.State),
		})
		return
	case db.JobStateSuccess:
		helpers.WriteJson(w, http.StatusOK, base{
			State:       string(data.State),
			Results:     data.Results,
			CompletedAt: data.CompletedAt.Time,
		})
		return
	default:
		h.Logger.Error("unknown state found in job", slog.Int64("job_id", id), "state", data.State)
		helpers.InternalServerError(w)
	}
}
