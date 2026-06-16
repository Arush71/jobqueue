package queue

import (
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

// Schedular is responsible for scheduling jobs
// and managing time
type Schedular struct {
	queue         *Queue
	scheduledJobs []*scheduledJobsT
	passer        chan struct{}
	mu            sync.RWMutex
}

func (schedular *Schedular) manageScheduling() {
	schedular.mu.RLock()
	if len(schedular.scheduledJobs) == 0 {
		schedular.mu.RUnlock()
		<-schedular.passer
		schedular.mu.RLock()
	}
	latestJobScheduled := schedular.scheduledJobs[0]
	schedular.mu.RUnlock()
	timer := time.NewTimer(time.Until(latestJobScheduled.scheduleAt))
	for {
		select {
		case <-schedular.passer:
			timer.Stop()
			schedular.mu.RLock()
			if len(schedular.scheduledJobs) == 0 {
				schedular.mu.RUnlock()
				continue
			}
			latestJobScheduled = schedular.scheduledJobs[0]
			schedular.mu.RUnlock()
			timer.Reset(time.Until(latestJobScheduled.scheduleAt))
		case <-timer.C:
			schedular.mu.Lock()
			for i, v := range schedular.scheduledJobs {
				if v == latestJobScheduled {
					if i < (len(schedular.scheduledJobs) - 1) {
						copy(schedular.scheduledJobs[i:], schedular.scheduledJobs[i+1:])
					}
					schedular.scheduledJobs[len(schedular.scheduledJobs)-1] = nil
					schedular.scheduledJobs = schedular.scheduledJobs[:len(schedular.scheduledJobs)-1]
					break
				}
			}
			schedular.mu.Unlock()
			schedular.enqueueJob(latestJobScheduled.jobID, latestJobScheduled.priority)
			schedular.mu.RLock()
			if len(schedular.scheduledJobs) > 0 {
				latestJobScheduled = schedular.scheduledJobs[0]
				timer.Reset(time.Until(latestJobScheduled.scheduleAt))
			}
			schedular.mu.RUnlock()
		}
	}
}

// InitSchedular inits the schedular
func InitSchedular(queue *Queue) *Schedular {
	schedule := &Schedular{
		scheduledJobs: make([]*scheduledJobsT, 0),
		passer:        make(chan struct{}, 1),
		queue:         queue,
	}
	go schedule.manageScheduling()
	return schedule
}

// ScheduleJob helps schedule jobs for the queue
func (schedular *Schedular) ScheduleJob(jobID int64, scheduleTime time.Time, priorityLevel Priority) {
	if time.Now().After(scheduleTime) {
		schedular.queue.logger.Warn("job schedultime already passed", slog.Int64("job_id", jobID))
		schedular.enqueueJob(jobID, priorityLevel)
		return
	}
	schedular.mu.Lock()
	schedular.scheduledJobs = append(schedular.scheduledJobs, &scheduledJobsT{
		jobID:      jobID,
		scheduleAt: scheduleTime,
		priority:   priorityLevel,
	})
	sort.Slice(schedular.scheduledJobs, func(i, j int) bool {
		return schedular.scheduledJobs[i].scheduleAt.Before(schedular.scheduledJobs[j].scheduleAt)
	})
	schedular.mu.Unlock()
	select {
	case schedular.passer <- struct{}{}:
	default:
	}
}

func (schedular *Schedular) enqueueJob(jobID int64, priorityLevel Priority) {
	if err := schedular.queue.EnqueueJob(jobID, priorityLevel, false); err != nil {
		if errors.Is(err, ErrQueueLimitReached) {
			base := 6
			waitTime := time.Now().Add(time.Duration(base+rand.IntN(base)) * time.Second)
			schedular.ScheduleJob(jobID, waitTime, priorityLevel)
			return
		}
		panic("unexpected error")
	}
}
