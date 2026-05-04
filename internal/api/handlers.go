// Package api provides HTTP handlers for interacting with the job queue system.
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/helpers"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/Arush71/jobqueue/internal/store"
	"github.com/Arush71/jobqueue/internal/types"
)

// Handler holds dependencies required for handling HTTP requests.
type Handler struct {
	DbQ   *db.Queries
	Queue *queue.Queue
}

// CreateJob handles job creation requests, validates input,
// persists the job, and enqueues it for processing.
func (h *Handler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req types.ReqJob
	if err := helpers.ReadJson(r, &req); err != nil {
		helpers.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := req.Validate(); err != nil {
		helpers.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	params, err := store.ToDBParams(req.Params)
	if err != nil {
		helpers.InternalServerError(w)
		return
	}
	jobID, err := h.DbQ.CreateJob(r.Context(), db.CreateJobParams{
		Type:      db.JobType(req.JobT),
		State:     db.JobStateQueued,
		ImagePath: req.ImagePath,
		Params:    params,
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
		ImagePath    string          `json:"image_path"`
		Params       json.RawMessage `json:"params"`
		RetryCounter int16           `json:"retry_counter"`
	}
	helpers.WriteJson(w, http.StatusOK, JobRes{
		JobID:        job.ID,
		JobType:      string(job.Type),
		State:        string(job.State),
		ImagePath:    job.ImagePath,
		Params:       job.Params,
		RetryCounter: job.RetryCounter,
	})
}
