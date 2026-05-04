// Package jobs defines the core job structures, states, and types
// used throughout the job queue system.
package jobs

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JT represents the type of job to be performed.
type JT string

// Supported job types.
const (
	Resize    JT = "resize"
	GrayScale JT = "grayscale"
	Compress  JT = "compress"
)

// JobState represents the lifecycle state of a job.
type JobState string

// Possible job states.
const (
	Queued     JobState = "queued"
	Processing JobState = "processing"
	Success    JobState = "success"
	Fail       JobState = "fail"
)

// ParamsT represents job-specific parameters as key-value pairs.
type ParamsT map[string]float64

// Job represents a unit of work to be processed by the system.
type Job struct {
	JobId        int64
	JobType      JT
	State        JobState
	ImagePath    string
	Params       ParamsT
	RetryCounter int16
}

// CreateJob initializes a new Job with the provided parameters
// and sets its initial state to queued.
func CreateJob(ImagePath string, JobType JT, Params ParamsT, id int64) *Job {
	return &Job{
		JobId:     id,
		JobType:   JobType,
		State:     Queued,
		ImagePath: ImagePath,
		Params:    Params,
	}
}

// UnmarshalJSON validates and parses a job type from JSON input.
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

// UnmarshalJSON validates and parses job parameters from JSON input,
// ensuring they are non-empty and normalized to lowercase keys.
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
