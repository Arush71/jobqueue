package api

import (
	"net/http"

	"github.com/Arush71/jobqueue/internal/helpers"
	"github.com/Arush71/jobqueue/internal/types"
)

type Handler struct {
	JobId *types.JobId
	Queue *types.Queue
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
	job := types.CreateJob(req.ImagePath, req.JobT, req.Params, id)
	h.Queue.AddJob(job)
	type res struct {
		Id int64 `json:"job_id"`
	}
	helpers.WriteJson(w, http.StatusCreated, res{
		Id: id,
	})
}

func (h *Handler) GetJobsById(w http.ResponseWriter, r *http.Request) {
	type reqT struct {
		JobId *int64 `json:"job_id"`
	}
	var req reqT
	if err := helpers.ReadJson(r, &req); err != nil {
		helpers.BadRequestError(w)
		return
	}
	if req.JobId == nil {
		helpers.Error(w, http.StatusBadRequest, "job id should be present and not null")
		return
	}
	job, ok := h.Queue.GetJobById(*req.JobId)
	if !ok {
		helpers.NotFoundError(w)
		return
	}
	type JobRes struct {
		JobId     int64         `json:"job_id"`
		JobType   string        `json:"job_type"`
		State     string        `json:"job_state"`
		ImagePath string        `json:"image_path"`
		Params    types.ParamsT `json:"params"`
	}
	helpers.WriteJson(w, http.StatusOK, JobRes{
		JobId:     job.JobId,
		JobType:   string(job.JobType),
		State:     string(job.State),
		ImagePath: job.ImagePath,
		Params:    job.Params,
	})
}
