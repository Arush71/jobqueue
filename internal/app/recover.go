package app

import (
	"context"
	"fmt"
	"time"

	"github.com/Arush71/jobqueue/internal/queue"
)

// Recover recovers the state from db and syncs it to in-memory
func (a *App) Recover() error {
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	err := a.dbQ.UpdateJobStateAtRestart(ctx)
	if err != nil {
		return fmt.Errorf("failed to change job state at recovery in db: %w", err)
	}
	jobIDs, err := a.dbQ.GetLeftJobs(ctx)
	if err != nil {
		return fmt.Errorf("failed to recover jobs from db after changing the state: %w", err)
	}
	for _, v := range jobIDs {
		if err := a.queue.EnqueueJob(v.ID, queue.Priority(v.JobPriority), true); err != nil {
			panic("got an unexpected error")
		}
	}
	return nil
}
