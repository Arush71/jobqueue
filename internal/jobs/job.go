package jobs

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type JT string

const (
	Resize    JT = "resize"
	GrayScale JT = "grayscale"
	Compress  JT = "compress"
)

type JobState string

const (
	Queued     JobState = "queued"
	Processing JobState = "processing"
	Success    JobState = "success"
	Fail       JobState = "fail"
)

type ParamsT map[string]float64

type Job struct {
	JobId     int64
	JobType   JT
	State     JobState
	ImagePath string
	Params    ParamsT
}

type JobId struct {
	Counter int64
	m       sync.Mutex
}

func (JId *JobId) GetNextId() int64 {
	JId.m.Lock()
	defer JId.m.Unlock()
	currentId := JId.Counter
	JId.Counter = currentId + 1
	return currentId
}

func CreateJob(ImagePath string, JobType JT, Params ParamsT, id int64) *Job {
	return &Job{
		JobId:     id,
		JobType:   JobType,
		State:     Queued,
		ImagePath: ImagePath,
		Params:    Params,
	}
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
