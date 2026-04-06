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
	q.jobCh <- a.Job
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

type Queue struct {
	qS    []*jobs.Job
	qM    map[int64]*jobs.Job
	jobCh chan *jobs.Job
	reqCh chan Request
}

func SetupQueue() *Queue {
	q := &Queue{
		qS:    make([]*jobs.Job, 0),
		qM:    make(map[int64]*jobs.Job),
		jobCh: make(chan *jobs.Job),
		reqCh: make(chan Request),
	}
	go q.loop()
	return q
}

func (q *Queue) AddJob(j *jobs.Job) {
	requeststr := AddReq{
		Job: j,
	}
	q.reqCh <- requeststr
}

func (q *Queue) GetJobById(id int64) (jobs.Job, bool) {
	sendChan := make(chan JobResult)
	getjobstr := GetJobReq{
		Id:       id,
		SendChan: sendChan,
	}
	q.reqCh <- getjobstr
	info := <-sendChan
	return info.Job, info.OK
}
func (q *Queue) GetWork() *jobs.Job {
	return <-q.jobCh
}

func (q *Queue) UpdateJob(j jobs.Job, state jobs.JobState) error {
	sendChan := make(chan error)
	updateStr := UpdateReq{
		Id:          j.JobId,
		State:       state,
		sendChannel: sendChan,
	}
	q.reqCh <- updateStr
	log.Println("job ", j.JobId, "->", state)
	return <-sendChan
}

func (q *Queue) loop() {
	for req := range q.reqCh {
		req.execute(q)
	}
}

// todo: fix the pipeline, make the new channel based system work.
