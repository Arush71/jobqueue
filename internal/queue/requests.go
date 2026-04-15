package queue

import (
	"fmt"

	"github.com/Arush71/jobqueue/internal/jobs"
)

type Request interface {
	execute(q *Queue)
}
type AddReq struct {
	JobId int64
}

func (a AddReq) execute(q *Queue) {
	q.qS = append(q.qS, a.JobId)
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
	Job_id int64
	OK     bool
}
type GetWorkS struct {
	SendChan chan<- GetQueueJob
}

func (g GetWorkS) execute(q *Queue) {
	len_job := len(q.qS)
	if len_job == 0 {
		g.SendChan <- GetQueueJob{
			OK: false,
		}
		return
	}
	job_id := q.qS[0]
	q.qS = q.qS[1:]
	g.SendChan <- GetQueueJob{
		OK:     true,
		Job_id: job_id,
	}
}
