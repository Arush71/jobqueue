package main

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type Queue struct {
	mu       sync.Mutex
	notEmpty *sync.Cond
	notFull  *sync.Cond
	items    []int
	capacity int
	closed   bool
}

func NewQueue(capacity int) *Queue {
	q := &Queue{capacity: capacity}
	q.notEmpty = sync.NewCond(&q.mu) // both Conds share the one mutex
	q.notFull = sync.NewCond(&q.mu)
	return q
}

func (q *Queue) Push(v int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == q.capacity { // ALWAYS wait in a loop, never an if
		q.notFull.Wait()
	}
	q.items = append(q.items, v)
	q.notEmpty.Signal() // wake one waiting consumer
}

func (q *Queue) Pop() (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.notEmpty.Wait()
	}
	if len(q.items) == 0 { // closed and drained
		return 0, false
	}
	v := q.items[0]
	q.items = q.items[1:]
	q.notFull.Signal() // wake one waiting producer
	return v, true
}

func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.notEmpty.Broadcast() // wake EVERY consumer so they can see closed and exit
}

func run() {
	q := NewQueue(5)
	var wg sync.WaitGroup
	var count, total atomic.Int64

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				v, ok := q.Pop()
				if !ok {
					return
				}
				count.Add(1)
				total.Add(int64(v))
			}
		}()
	}

	for v := 1; v <= 30; v++ {
		q.Push(v)
	}
	q.Close()

	wg.Wait()
	fmt.Println("consumed:", count.Load(), "sum:", total.Load()) // consumed: 30 sum: 465
}
