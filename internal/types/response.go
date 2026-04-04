package types

import (
	"fmt"
	"github.com/Arush71/jobqueue/internal/jobs"
)

type ReqJob struct {
	JobT      jobs.JT      `json:"job_type"`
	ImagePath string       `json:"image_path"`
	Params    jobs.ParamsT `json:"params"`
}

func (req *ReqJob) Validate() error {
	if req.ImagePath == "" {
		return fmt.Errorf("image path should not be empty")
	}
	switch req.JobT {
	case jobs.Resize:
		if v, ok := req.Params["width"]; !ok || v <= 0 {
			return fmt.Errorf("must have width and be over 0")
		}
		if v, ok := req.Params["height"]; !ok || v <= 0 {
			return fmt.Errorf("must have height and be over 0")

		}
		if len(req.Params) > 2 {

			return fmt.Errorf("params must not have any extra fields")
		}
	case jobs.Compress:
		q, ok := req.Params["quantity"]
		if !ok {
			return fmt.Errorf("must have quantity and be over 1 and under 100")
		}
		if q < 1 || q > 100 {
			return fmt.Errorf("must have quantity and be over 1 and under 100")
		}
		if len(req.Params) > 1 {

			return fmt.Errorf("params must not have any extra fields")
		}
	case jobs.GrayScale:
		q, ok := req.Params["quality"]
		if !ok {
			return fmt.Errorf("must have quality and be over 0.1 and under 1")
		}
		if q < 0.1 || q > 1 {
			return fmt.Errorf("must have quality and be over 0.1 and under 1")
		}
		if len(req.Params) > 1 {

			return fmt.Errorf("params must not have any extra fields")
		}
	default:

		return fmt.Errorf("job type is required")
	}
	return nil
}
