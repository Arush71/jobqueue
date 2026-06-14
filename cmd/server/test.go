package main

import (
	"context"
	"fmt"
	"sync"
)

type Job struct {
	ID      string
	Payload []byte
}

type Result struct {
	JobID string
	Err   error
}

func processJob(job Job) error {
	fmt.Printf("  processed job %s (%d bytes)\n", job.ID, len(job.Payload))
	return nil
}

func NewPool(ctx context.Context, workers int, jobs <-chan Job) <-chan Result {
	results := make(chan Result, workers)
	var wg sync.WaitGroup

	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for {
				select {
				case job, ok := <-jobs:
					if !ok {
						return // jobs channel closed
					}
					err := processJob(job)
					select {
					case results <- Result{JobID: job.ID, Err: err}:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

func run() {
	events := []struct {
		ID   string
		Data []byte
	}{
		{"evt-1", []byte(`{"type":"click"}`)},
		{"evt-2", []byte(`{"type":"view"}`)},
		{"evt-3", []byte(`{"type":"buy"}`)},
	}

	jobs := make(chan Job, 100)
	results := NewPool(context.Background(), 3, jobs)

	go func() {
		defer close(jobs)
		for _, event := range events {
			jobs <- Job{ID: event.ID, Payload: event.Data}
		}
	}()

	for r := range results {
		if r.Err != nil {
			fmt.Printf("job %s failed: %v\n", r.JobID, r.Err)
		}
	}
}
