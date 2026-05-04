// Package queue provides an in-memory job queue with safe concurrent access.
package queue

// Queue represents an in-memory job queue that coordinates
// producers and workers using channels.
type Queue struct {
	qS       []int64
	notifyCh chan struct{}
	reqCh    chan Request
}

// SetupQueue initializes a new Queue instance and starts
// its internal event loop for handling requests.
func SetupQueue() *Queue {
	q := &Queue{
		qS:       make([]int64, 0),
		notifyCh: make(chan struct{}, 1),
		reqCh:    make(chan Request),
	}
	go q.loop()
	return q
}

// EnqueueJob adds a new job ID to the queue for processing.
func (q *Queue) EnqueueJob(jID int64) {
	requeststr := AddReq{
		JobID: jID,
	}
	q.reqCh <- requeststr
}

// GetWork retrieves a job ID from the queue, blocking until
// a job is available.
func (q *Queue) GetWork() int64 {
	for {
		sendChan := make(chan GetQueueJob)
		getJob := GetWorkS{
			SendChan: sendChan,
		}
		q.reqCh <- getJob
		info := <-sendChan
		if info.OK {
			return info.JobID
		}
		<-q.notifyCh
	}
}

// loop runs the internal event loop that processes queue requests.
func (q *Queue) loop() {
	for req := range q.reqCh {
		req.execute(q)
	}
}
