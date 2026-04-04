package queue

import (
	"log"
	"sync"

	"github.com/Arush71/jobqueue/internal/jobs"
)

type Queue struct {
	Qs []*jobs.Job
	Qm map[int64]*jobs.Job
	mu sync.RWMutex
}

func SetupQ() *Queue {
	return &Queue{
		Qs: make([]*jobs.Job, 0),
		Qm: make(map[int64]*jobs.Job),
	}
}

func (q *Queue) AddJob(j *jobs.Job) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.Qs = append(q.Qs, j)
	q.Qm[j.JobId] = j
}

func (q *Queue) GetJobById(id int64) (*jobs.Job, bool) {
	q.mu.RLock()
	value, ok := q.Qm[id]
	q.mu.RUnlock()
	return value, ok
}
func (q *Queue) GetNextJob() (*jobs.Job, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	for i := range q.Qs {
		if q.Qs[i].State == jobs.Queued {
			return q.Qs[i], true
		}
	}
	return nil, false
}

func (q *Queue) UpdateJob(j *jobs.Job, state jobs.JobState) {
	q.mu.Lock()
	defer q.mu.Unlock()
	j.State = state
	log.Println("job ", j.JobId, "->", state)
}
