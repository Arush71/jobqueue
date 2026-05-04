package queue

// Request represents a queue operation that can be executed
// within the queue's internal event loop.
type Request interface {
	execute(q *Queue)
}

// AddReq represents a request to add a job ID to the queue.
type AddReq struct {
	JobID int64
}

// execute appends the job ID to the queue and notifies
// waiting workers that a new job is available.
func (a AddReq) execute(q *Queue) {
	q.qS = append(q.qS, a.JobID)
	select {
	case q.notifyCh <- struct{}{}:
	default:
	}
}

// GetQueueJob represents the result of a dequeue operation,
// indicating whether a job was available and its ID.
type GetQueueJob struct {
	JobID int64
	OK    bool
}

// GetWorkS represents a request to retrieve a job from the queue.
type GetWorkS struct {
	SendChan chan<- GetQueueJob
}

// execute attempts to retrieve a job from the queue and sends
// the result back through the provided channel.
func (g GetWorkS) execute(q *Queue) {
	lenJob := len(q.qS)
	if lenJob == 0 {
		g.SendChan <- GetQueueJob{
			OK: false,
		}
		return
	}
	jobID := q.qS[0]
	q.qS = q.qS[1:]
	g.SendChan <- GetQueueJob{
		OK:    true,
		JobID: jobID,
	}
}
