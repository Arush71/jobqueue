package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/Arush71/jobqueue/internal/helpers"
	"github.com/Arush71/jobqueue/internal/jobs"
	"github.com/Arush71/jobqueue/internal/queue"
	"github.com/Arush71/jobqueue/internal/types"
)

type Handler struct {
	JobId *jobs.JobId
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
	id := h.JobId.GetNextId()
	job := jobs.CreateJob(req.ImagePath, req.JobT, req.Params, id)
	h.Queue.AddJob(job)
	log.Println("job created: id and type are", job.JobId, "and", job.JobType)
	type res struct {
		Id int64 `json:"job_id"`
	}
	helpers.WriteJson(w, http.StatusCreated, res{
		Id: id,
	})
}

func (h *Handler) GetJobsById(w http.ResponseWriter, r *http.Request) {
	id_str := r.PathValue("id")
	id, err := strconv.ParseInt(id_str, 10, 64)
	if err != nil {
		helpers.Error(w, http.StatusBadRequest, "inavlid jod id:"+id_str)
		return
	}
	job, ok := h.Queue.GetJobById(id)
	if !ok {
		helpers.NotFoundError(w)
		return
	}
	type JobRes struct {
		JobId     int64        `json:"job_id"`
		JobType   string       `json:"job_type"`
		State     string       `json:"job_state"`
		ImagePath string       `json:"image_path"`
		Params    jobs.ParamsT `json:"params"`
	}
	helpers.WriteJson(w, http.StatusOK, JobRes{
		JobId:     job.JobId,
		JobType:   string(job.JobType),
		State:     string(job.State),
		ImagePath: job.ImagePath,
		Params:    job.Params,
	})
}
