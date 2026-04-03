package types

import "sync"

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

type Queue struct {
	Qs []*Job
	Qm map[int64]*Job
}

func SetupQ() *Queue {
	return &Queue{
		Qs: make([]*Job, 0),
		Qm: make(map[int64]*Job),
	}
}

func (q *Queue) AddJob(j *Job) {
	q.Qs = append(q.Qs, j)
	q.Qm[j.JobId] = j
}

func (q *Queue) GetJobById(id int64) (*Job, bool) {
	value, ok := q.Qm[id]
	return value, ok
}
