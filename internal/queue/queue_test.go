package queue

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestPriorityValidation(t *testing.T) {
	for _, priority := range []Priority{High, Normal, Low} {
		assert.True(t, priority.IsValid())
	}
	assert.False(t, Priority("urgent").IsValid())
}

func TestQueueStrictPriorityAndFIFO(t *testing.T) {
	q := SetupQueue(testLogger(), 1)
	for _, item := range []struct {
		id       int64
		priority Priority
	}{{1, Low}, {2, High}, {3, Normal}, {4, High}, {5, Low}} {
		require.NoError(t, q.EnqueueJob(item.id, item.priority, false))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for i, want := range []int64{2, 4, 3, 1, 5} {
		got, err := q.GetWork(ctx)
		require.NoError(t, err, "item %d", i)
		assert.Equal(t, want, got)
	}
}

func TestQueueLimitsAndForce(t *testing.T) {
	for _, tc := range []struct {
		priority Priority
		limit    int
	}{{High, 32}, {Normal, 64}, {Low, 128}} {
		t.Run(string(tc.priority), func(t *testing.T) {
			q := SetupQueue(testLogger(), 1)
			for i := 0; i < tc.limit; i++ {
				require.NoError(t, q.EnqueueJob(int64(i), tc.priority, false))
			}
			assert.ErrorIs(t, q.CheckCapacity(tc.priority), ErrQueueLimitReached)
			assert.ErrorIs(t, q.EnqueueJob(1000, tc.priority, false), ErrQueueLimitReached)
			require.NoError(t, q.EnqueueJob(1001, tc.priority, true))
			assert.Equal(t, int32(tc.limit+1), q.queueSizePerPriority[tc.priority.idx()].Load())
		})
	}
}

func TestQueueRejectsInvalidPriority(t *testing.T) {
	q := SetupQueue(testLogger(), 1)
	assert.Error(t, q.CheckCapacity(Priority("urgent")))
	assert.Error(t, q.EnqueueJob(1, Priority("urgent"), false))
}

func TestGetWorkBlocksThenWakes(t *testing.T) {
	q := SetupQueue(testLogger(), 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result := make(chan int64, 1)
	go func() {
		id, err := q.GetWork(ctx)
		if err == nil {
			result <- id
		}
	}()

	select {
	case <-result:
		t.Fatal("GetWork returned before work was available")
	case <-time.After(20 * time.Millisecond):
	}
	require.NoError(t, q.EnqueueJob(42, Normal, false))
	select {
	case got := <-result:
		assert.Equal(t, int64(42), got)
	case <-ctx.Done():
		t.Fatal("worker was not notified")
	}
}

func TestGetWorkCancellation(t *testing.T) {
	q := SetupQueue(testLogger(), 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := q.GetWork(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestQueueConcurrentProducersAndConsumers(t *testing.T) {
	const total = 500
	q := SetupQueue(testLogger(), 16)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var producers sync.WaitGroup
	for i := 0; i < total; i++ {
		i := i
		producers.Go(func() {
			require.NoError(t, q.EnqueueJob(int64(i), Normal, true))
		})
	}
	producers.Wait()

	results := make(chan int64, total)
	var consumers sync.WaitGroup
	for range 8 {
		consumers.Go(func() {
			for {
				id, err := q.GetWork(ctx)
				if err != nil {
					return
				}
				results <- id
				if len(results) == total {
					cancel()
					return
				}
			}
		})
	}
	consumers.Wait()
	close(results)

	seen := make(map[int64]bool, total)
	for id := range results {
		assert.False(t, seen[id], "duplicate job %d", id)
		seen[id] = true
	}
	assert.Len(t, seen, total)
}
