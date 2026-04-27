package queue

type Queue struct {
	qS       []int64
	notifyCh chan struct{}
	reqCh    chan Request
}

func SetupQueue() *Queue {
	q := &Queue{
		qS:       make([]int64, 0),
		notifyCh: make(chan struct{}, 1),
		reqCh:    make(chan Request),
	}
	go q.loop()
	return q
}

func (q *Queue) EnqueueJob(j_id int64) {
	requeststr := AddReq{
		JobId: j_id,
	}
	q.reqCh <- requeststr
}

func (q *Queue) GetWork() int64 {
	for {
		sendChan := make(chan GetQueueJob)
		getJob := GetWorkS{
			SendChan: sendChan,
		}
		q.reqCh <- getJob
		info := <-sendChan
		if info.OK {
			return info.Job_id
		}
		<-q.notifyCh
	}

}

func (q *Queue) loop() {
	for req := range q.reqCh {
		req.execute(q)
	}
}
