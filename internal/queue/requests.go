package queue

import (
	"log/slog"

	"github.com/Arush71/jobqueue/internal/metrics"
)

// Request represents a queue operation that can be executed
// within the queue's internal event loop.
type Request interface {
	execute(q *Queue)
}

// AddReq represents a request to add a job ID to the queue.
type AddReq struct {
	JobID    int64
	priority Priority
}

// execute appends the job ID to the queue and notifies
// waiting workers that a new job is available.
func (a AddReq) execute(q *Queue) {
	switch a.priority {
	case High:
		q.priorityQueues.high = append(q.priorityQueues.high, a.JobID)
		q.numOfJob[0].Add(1)
	case Normal:
		q.priorityQueues.normal = append(q.priorityQueues.normal, a.JobID)
		q.numOfJob[1].Add(1)
	case Low:
		q.priorityQueues.low = append(q.priorityQueues.low, a.JobID)
		q.numOfJob[2].Add(1)
	default:
		q.logger.Error("invalid job priority", slog.Int64("job_id", a.JobID), slog.String("priority", string(a.priority)))
		return
	}
	metrics.QueueDepth.Inc()
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
	queue, ok := q.priorityQueues.selectQueue()
	if !ok {
		g.SendChan <- GetQueueJob{
			OK: false,
		}
		return
	}
	jobID := (*queue)[0]
	*queue = (*queue)[1:]
	// abandoning the internal array to better manage memory
	if len(*queue) == 0 {
		*queue = make(queueType, 0)
	}
	g.SendChan <- GetQueueJob{
		OK:    true,
		JobID: jobID,
	}
}
