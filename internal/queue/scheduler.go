package queue

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sort"
	"sync"
	"time"
)

type scheduledJobsT struct {
	jobID      int64
	scheduleAt time.Time
	priority   Priority
}

// Scheduler is responsible for scheduling jobs
// and managing time
type Scheduler struct {
	queue          *Queue
	scheduledJobs  []*scheduledJobsT
	wakeCh         chan struct{}
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
	stopOnce       sync.Once
	fullRetryDelay func() time.Duration
}

func (scheduler *Scheduler) manageScheduling() {
	defer close(scheduler.done)
	var timer *time.Timer
	// A nil channel disables the timer case while still allowing wake-up
	// notifications and graceful shutdown.
	var timerCh <-chan time.Time
	var timedJob *scheduledJobsT
	for {
		scheduler.mu.RLock()
		if len(scheduler.scheduledJobs) > 0 {
			timedJob = scheduler.scheduledJobs[0]
			wait := time.Until(timedJob.scheduleAt)
			if timer == nil {
				timer = time.NewTimer(wait)
			} else {
				timer.Stop()
				timer.Reset(wait)
			}
			timerCh = timer.C
		} else {
			timedJob = nil
			timerCh = nil
		}
		scheduler.mu.RUnlock()

		select {
		case <-scheduler.ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-scheduler.wakeCh:
			continue
		case <-timerCh:
			// earlier job may have become index zero while the timer fired.
			if !scheduler.removeScheduledJob(timedJob) {
				continue
			}
			scheduler.enqueueJob(timedJob.jobID, timedJob.priority)
		}
	}
}

func (scheduler *Scheduler) removeScheduledJob(target *scheduledJobsT) bool {
	scheduler.mu.Lock()
	defer scheduler.mu.Unlock()
	for i, job := range scheduler.scheduledJobs {
		if job != target {
			continue
		}
		copy(scheduler.scheduledJobs[i:], scheduler.scheduledJobs[i+1:])
		scheduler.scheduledJobs[len(scheduler.scheduledJobs)-1] = nil
		scheduler.scheduledJobs = scheduler.scheduledJobs[:len(scheduler.scheduledJobs)-1]
		return true
	}
	return false
}

// InitScheduler initializes the scheduler.
func InitScheduler(queue *Queue) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	scheduler := &Scheduler{
		scheduledJobs: make([]*scheduledJobsT, 0),
		wakeCh:        make(chan struct{}, 1),
		queue:         queue,
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
		fullRetryDelay: func() time.Duration {
			base := 6
			return time.Duration(base+rand.IntN(base)) * time.Second
		},
	}
	go scheduler.manageScheduling()
	return scheduler
}

// Stop terminates the scheduler's background goroutine.
func (scheduler *Scheduler) Stop() {
	scheduler.stopOnce.Do(scheduler.cancel)
	<-scheduler.done
}

// ScheduleJob helps schedule jobs for the queue
func (scheduler *Scheduler) ScheduleJob(jobID int64, scheduleTime time.Time, priorityLevel Priority) {
	if time.Now().After(scheduleTime) {
		scheduler.queue.logger.Warn("job schedule time already passed", slog.Int64("job_id", jobID))
		scheduler.enqueueJob(jobID, priorityLevel)
		return
	}
	scheduler.mu.Lock()
	scheduler.scheduledJobs = append(scheduler.scheduledJobs, &scheduledJobsT{
		jobID:      jobID,
		scheduleAt: scheduleTime,
		priority:   priorityLevel,
	})
	sort.Slice(scheduler.scheduledJobs, func(i, j int) bool {
		return scheduler.scheduledJobs[i].scheduleAt.Before(scheduler.scheduledJobs[j].scheduleAt)
	})
	scheduler.mu.Unlock()
	select {
	case scheduler.wakeCh <- struct{}{}:
	default:
	}
}

func (scheduler *Scheduler) enqueueJob(jobID int64, priorityLevel Priority) {
	if err := scheduler.queue.EnqueueJob(jobID, priorityLevel, false); err != nil {
		if errors.Is(err, ErrQueueLimitReached) {
			waitTime := time.Now().Add(scheduler.fullRetryDelay())
			scheduler.ScheduleJob(jobID, waitTime, priorityLevel)
			return
		}
		panic("unexpected error")
	}
}
