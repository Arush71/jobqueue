// Package queue provides an in-memory job queue with safe concurrent access.
package queue

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

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

// IsValid reports wether the priority is valid or not
func (p Priority) IsValid() bool {
	switch p {
	case High, Normal, Low:
		return true
	default:
		return false
	}
}

func (p Priority) idx() int {
	switch p {
	case High:
		return 0
	case Normal:
		return 1
	case Low:
		return 2
	default:
		// idx() should only ever be called on a invalid priority, therefore panic
		panic("unknown priority lvl")
	}
}

// ErrQueueLimitReached error for when queue has reached its limit
var ErrQueueLimitReached = errors.New("queue limit reached, try again later")

// IsQueueFree reports wether queue is free or not
func (q *Queue) IsQueueFree(p Priority) error {
	if !p.IsValid() {
		q.logger.Error("invalid priority", "priority", p)
		return errors.New("invalid priority")
	}
	switch p {
	case High:
		if q.queueSizePerPriority[0].Load() >= 32 {
			return ErrQueueLimitReached
		}
	case Normal:
		if q.queueSizePerPriority[1].Load() >= 64 {
			return ErrQueueLimitReached
		}
	case Low:
		if q.queueSizePerPriority[2].Load() >= 128 {
			return ErrQueueLimitReached
		}
	default:
		panic("unknown priority after verifying")
	}
	return nil
}

type queueType []int64

type priorityQueuesT struct {
	high   queueType
	normal queueType
	low    queueType
}

// Queue represents an in-memory job queue that coordinates
// producers and workers using channels.
type Queue struct {
	priorityQueues       priorityQueuesT
	notifyCh             chan struct{}
	logger               *slog.Logger
	mu                   sync.Mutex
	queueSizePerPriority [3]atomic.Int32
}

// SetupQueue initializes a new Queue instance and starts
// its internal event loop for handling requests.
func SetupQueue(logger *slog.Logger, numOfWorkers int) *Queue {
	q := &Queue{
		priorityQueues: priorityQueuesT{
			high:   make(queueType, 0),
			normal: make(queueType, 0),
			low:    make(queueType, 0),
		},
		notifyCh: make(chan struct{}, numOfWorkers),
		logger:   logger,
	}
	return q
}

// EnqueueJob adds a new job ID to the queue for processing.
func (q *Queue) EnqueueJob(jID int64, queuePriority Priority, force bool) error {
	if !force {
		if err := q.IsQueueFree(queuePriority); err != nil {
			return err
		}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if !force {
		if err := q.IsQueueFree(queuePriority); err != nil {
			return err
		}
	}
	switch queuePriority {
	case High:
		q.priorityQueues.high = append(q.priorityQueues.high, jID)
	case Normal:
		q.priorityQueues.normal = append(q.priorityQueues.normal, jID)
	case Low:
		q.priorityQueues.low = append(q.priorityQueues.low, jID)
	}
	q.queueSizePerPriority[queuePriority.idx()].Add(1)
	metrics.QueueDepth.Inc()
	select {
	case q.notifyCh <- struct{}{}:
	default:
	}
	return nil
}

// TODO: current design allows for starvation.
func (qs *priorityQueuesT) selectQueue() (*queueType, Priority, bool) {
	if len(qs.high) > 0 {
		return &qs.high, High, true
	}
	if len(qs.normal) > 0 {
		return &qs.normal, Normal, true
	}
	if len(qs.low) > 0 {
		return &qs.low, Low, true
	}
	return nil, "", false
}

// GetWork retrieves a job ID from the queue, blocking until
// a job is available.
func (q *Queue) GetWork(ctx context.Context) (int64, error) {
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		q.mu.Lock()
		queue, priority, ok := q.priorityQueues.selectQueue()
		if !ok {
			q.mu.Unlock()
			select {
			case <-q.notifyCh:
				continue
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
		jobID := (*queue)[0]
		*queue = (*queue)[1:]
		q.queueSizePerPriority[priority.idx()].Add(-1)
		q.mu.Unlock()
		metrics.QueueDepth.Dec()
		return jobID, nil
	}
}
