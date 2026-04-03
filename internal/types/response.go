package types

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ReqJob struct {
	JobT      JT      `json:"job_type"`
	ImagePath string  `json:"image_path"`
	Params    ParamsT `json:"params"`
}

func (req *ReqJob) Validate() error {
	if req.ImagePath == "" {
		return fmt.Errorf("image path should not be empty")
	}
	switch req.JobT {
	case Resize:
		if v, ok := req.Params["width"]; !ok || v <= 0 {
			return fmt.Errorf("must have width and be over 0")
		}
		if v, ok := req.Params["height"]; !ok || v <= 0 {
			return fmt.Errorf("must have height and be over 0")

		}
		if len(req.Params) > 2 {

			return fmt.Errorf("params must not have any extra fields")
		}
	case Compress:
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
	case GrayScale:
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

func (j *JT) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("job type must be a string")
	}
	switch JT(s) {
	case Resize, GrayScale, Compress:
		*j = JT(s)
		return nil
	default:
		return fmt.Errorf("invalid job type")
	}
}

func (p *ParamsT) UnmarshalJSON(data []byte) error {
	var s map[string]float64
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("paramaters must be a map")
	}
	if len(s) == 0 {
		return fmt.Errorf("paramaters must not be empty")
	}
	*p = make(ParamsT)
	for k, v := range s {
		(*p)[strings.ToLower(k)] = v
	}
	return nil
}
