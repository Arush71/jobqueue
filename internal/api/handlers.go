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

type Handler struct {
	DbQ   *db.Queries
	Queue *queue.Queue
}

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
	job_id, err := h.DbQ.CreateJob(r.Context(), db.CreateJobParams{
		Type:      db.JobType(req.JobT),
		State:     db.JobStateQueued,
		ImagePath: req.ImagePath,
		Params:    params,
	})
	if err != nil {
		helpers.InternalServerError(w)
		return
	}
	h.Queue.EnqueueJob(job_id)
	log.Println("Job created with id and enqueued:", job_id)
	type res struct {
		Id int64 `json:"job_id"`
	}
	helpers.WriteJson(w, http.StatusCreated, res{
		Id: job_id,
	})
}

func (h *Handler) GetJobsById(w http.ResponseWriter, r *http.Request) {
	id_str := r.PathValue("id")
	id, err := strconv.ParseInt(id_str, 10, 64)
	if err != nil {
		helpers.Error(w, http.StatusBadRequest, "inavlid jod id:"+id_str)
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
		JobId     int64           `json:"job_id"`
		JobType   string          `json:"job_type"`
		State     string          `json:"job_state"`
		ImagePath string          `json:"image_path"`
		Params    json.RawMessage `json:"params"`
	}
	helpers.WriteJson(w, http.StatusOK, JobRes{
		JobId:     job.ID,
		JobType:   string(job.Type),
		State:     string(job.State),
		ImagePath: job.ImagePath,
		Params:    job.Params,
	})
}
