// Package api provides HTTP handlers for interacting with the job queue system.
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/helpers"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/queue"
)

// Handler holds dependencies required for handling HTTP requests.
type Handler struct {
	DbQ   *db.Queries
	Queue *queue.Queue
}

// CreateJob handles job creation requests, validates input,
// persists the job, and enqueues it for processing.
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req jobs.JobHandlerTypeHTTP
	if err := helpers.ReadJson(r, &req); err != nil {
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
	jobID, err := h.DbQ.CreateJob(r.Context(), db.CreateJobParams{
		JobType: req.JobType,
		State:   db.JobStateQueued,
		Payload: req.Payload,
	})
	if err != nil {
		helpers.InternalServerError(w)
		return
	}
	h.Queue.EnqueueJob(jobID)
	log.Println("Job created with id and enqueued:", jobID)
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
		helpers.Error(w, http.StatusBadRequest, "inavlid jod id:"+idStr)
		return
	}
	job, err := h.DbQ.GetJobById(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.NotFoundError(w)
			return
		}
		log.Println("db error:", err)
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
		helpers.Error(w, http.StatusBadRequest, "inavlid jod id:"+idStr)
		return
	}
	data, err := h.DbQ.GetResultFromJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			helpers.NotFoundError(w)
			return
		}
		log.Println("db error:", err)
		helpers.InternalServerError(w)
		return
	}
	type base struct {
		Results     []byte    `json:"results,omitempty"`
		Error       string    `json:"error,omitempty"`
		State       string    `json:"state"`
		CompletedAt time.Time `json:"completed_at,omitempty"`
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
		log.Printf("[WARNING] Job %d has unknown state: %s", id, data.State)
		helpers.InternalServerError(w)
	}
}
