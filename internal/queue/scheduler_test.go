package queue

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchedulerImmediateAndFutureJobs(t *testing.T) {
	q := SetupQueue(testLogger(), 1)
	s := InitScheduler(q)
	t.Cleanup(s.Stop)

	s.ScheduleJob(1, time.Now().Add(-time.Millisecond), Normal)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	id, err := q.GetWork(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)

	s.ScheduleJob(2, time.Now().Add(50*time.Millisecond), High)
	earlyCtx, earlyCancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer earlyCancel()
	_, err = q.GetWork(earlyCtx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	id, err = q.GetWork(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), id)
}

func TestSchedulerReordersWhenEarlierJobArrives(t *testing.T) {
	q := SetupQueue(testLogger(), 1)
	s := InitScheduler(q)
	t.Cleanup(s.Stop)

	s.ScheduleJob(1, time.Now().Add(200*time.Millisecond), Normal)
	s.ScheduleJob(2, time.Now().Add(20*time.Millisecond), Normal)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	id, err := q.GetWork(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), id)
}

func TestSchedulerTimerRemovesTheJobItWasArmedFor(t *testing.T) {
	now := time.Now()
	timedJob := &scheduledJobsT{jobID: 1, scheduleAt: now.Add(time.Second), priority: Normal}
	newEarlierJob := &scheduledJobsT{jobID: 2, scheduleAt: now, priority: High}
	s := &Scheduler{
		scheduledJobs: []*scheduledJobsT{newEarlierJob, timedJob},
	}

	require.True(t, s.removeScheduledJob(timedJob))
	require.Len(t, s.scheduledJobs, 1)
	assert.Same(t, newEarlierJob, s.scheduledJobs[0])
}

func TestSchedulerReschedulesWhenQueueIsFull(t *testing.T) {
	q := SetupQueue(testLogger(), 1)
	for i := range 32 {
		require.NoError(t, q.EnqueueJob(int64(i), High, false))
	}
	s := InitScheduler(q)
	s.fullRetryDelay = func() time.Duration { return 15 * time.Millisecond }
	t.Cleanup(s.Stop)
	s.ScheduleJob(99, time.Now().Add(-time.Millisecond), High)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for range 32 {
		_, err := q.GetWork(ctx)
		require.NoError(t, err)
	}
	id, err := q.GetWork(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(99), id)
}

func TestSchedulerStopPreventsDelivery(t *testing.T) {
	q := SetupQueue(testLogger(), 1)
	s := InitScheduler(q)
	s.ScheduleJob(1, time.Now().Add(30*time.Millisecond), Normal)
	s.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	_, err := q.GetWork(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
