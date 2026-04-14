package queue

import (
	"log"

	"github.com/Arush71/jobqueue/internal/jobs"
)

type Queue struct {
	qS       []*jobs.Job
	qM       map[int64]*jobs.Job
	notifyCh chan struct{}
	reqCh    chan Request
}

func SetupQueue() *Queue {
	q := &Queue{
		qS:       make([]*jobs.Job, 0),
		qM:       make(map[int64]*jobs.Job),
		notifyCh: make(chan struct{}, 1),
		reqCh:    make(chan Request),
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
func (q *Queue) GetWork() jobs.Job {
	for {
		sendChan := make(chan GetQueueJob)
		getJob := GetWorkS{
			SendChan: sendChan,
		}
		q.reqCh <- getJob
		info := <-sendChan
		if info.OK {
			return info.Job
		}
		<-q.notifyCh
	}

}

func (q *Queue) UpdateJob(jId int64, state jobs.JobState) error {
	sendChan := make(chan error)
	updateStr := UpdateReq{
		Id:          jId,
		State:       state,
		sendChannel: sendChan,
	}
	q.reqCh <- updateStr
	log.Println("job ", jId, "->", state)
	return <-sendChan
}

func (q *Queue) loop() {
	for req := range q.reqCh {
		req.execute(q)
	}
}
