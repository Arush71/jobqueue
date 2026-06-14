// Package queue provides an in-memory job queue with safe concurrent access.
package queue

import (
	"context"
	"log/slog"

	"github.com/Arush71/jobqueue/internal/metrics"
)

// Priority is the type of queue priority
type Priority string

// Exported types
const (
	High   Priority = "high"
	Normal Priority = "normal"
	Low    Priority = "low"
)

type queueType []int64

type priorityQueuesT struct {
	high   queueType
	normal queueType
	low    queueType
}

// Queue represents an in-memory job queue that coordinates
// producers and workers using channels.
type Queue struct {
	priorityQueues priorityQueuesT
	notifyCh       chan struct{}
	reqCh          chan Request
	logger         *slog.Logger
}

// SetupQueue initializes a new Queue instance and starts
// its internal event loop for handling requests.
func SetupQueue(logger *slog.Logger) *Queue {
	q := &Queue{
		priorityQueues: priorityQueuesT{
			high:   make(queueType, 0),
			normal: make(queueType, 0),
			low:    make(queueType, 0),
		},
		notifyCh: make(chan struct{}, 1),
		reqCh:    make(chan Request),
		logger:   logger,
	}
	go q.loop()
	return q
}

// EnqueueJob adds a new job ID to the queue for processing.
func (q *Queue) EnqueueJob(jID int64, queuePriority Priority) {
	requeststr := AddReq{
		JobID:    jID,
		priority: queuePriority,
	}
	q.reqCh <- requeststr
}

func (qs *priorityQueuesT) selectQueue() (*queueType, bool) {
	if len(qs.high) > 0 {
		return &qs.high, true
	}
	if len(qs.normal) > 0 {
		return &qs.normal, true
	}
	if len(qs.low) > 0 {
		return &qs.low, true
	}
	return nil, false
}

// GetWork retrieves a job ID from the queue, blocking until
// a job is available.
func (q *Queue) GetWork(ctx context.Context) (int64, error) {
	sendChan := make(chan GetQueueJob)
	for {
		getJob := GetWorkS{
			SendChan: sendChan,
		}
		select {
		case q.reqCh <- getJob:
		case <-ctx.Done():
			return 0, ctx.Err()
		}
		info := <-sendChan
		if info.OK {
			metrics.QueueDepth.Dec()
			return info.JobID, nil
		}
		select {
		case <-q.notifyCh:
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
}

// loop runs the internal event loop that processes queue requests.
func (q *Queue) loop() {
	for req := range q.reqCh {
		req.execute(q)
	}
}
