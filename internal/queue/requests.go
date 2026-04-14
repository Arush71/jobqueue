package queue

import (
	"fmt"
	"log"

	"github.com/Arush71/jobqueue/internal/jobs"
)

type Request interface {
	execute(q *Queue)
}
type AddReq struct {
	Job *jobs.Job
}

func (a AddReq) execute(q *Queue) {
	q.qS = append(q.qS, a.Job)
	q.qM[a.Job.JobId] = a.Job
	select {
	case q.notifyCh <- struct{}{}:
	default:
	}
}

type UpdateReq struct {
	Id          int64
	State       jobs.JobState
	sendChannel chan<- error
}

func (uR UpdateReq) execute(q *Queue) {
	value, ok := q.qM[uR.Id]
	if !ok {
		uR.sendChannel <- fmt.Errorf("worker state update error: job of %d not found", uR.Id)
		return
	}
	value.State = uR.State
	uR.sendChannel <- nil
}

type JobResult struct {
	Job jobs.Job
	OK  bool
}
type GetJobReq struct {
	Id       int64
	SendChan chan<- JobResult
}

func (gJ GetJobReq) execute(q *Queue) {
	value, ok := q.qM[gJ.Id]
	if !ok {
		gJ.SendChan <- JobResult{
			OK: false,
		}
		return
	}
	gJ.SendChan <- JobResult{
		OK:  true,
		Job: *value,
	}
}

type GetQueueJob struct {
	Job jobs.Job
	OK  bool
}
type GetWorkS struct {
	SendChan chan<- GetQueueJob
}

func (g GetWorkS) execute(q *Queue) {
	for _, j := range q.qS {
		if j.State == jobs.Queued {
			j.State = jobs.Processing
			log.Println("job ", j.JobId, "->", jobs.Processing)
			g.SendChan <- GetQueueJob{
				Job: *j,
				OK:  true,
			}
			return
		}
	}
	g.SendChan <- GetQueueJob{
		OK: false,
	}
}
