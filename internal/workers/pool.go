package workers

import (
	"context"
	"log/slog"
	"sync"

	"github.com/Arush71/jobqueue/internal/db"
	"github.com/Arush71/jobqueue/internal/queue"
)

type Pool struct {
	numWorkers int
	start      sync.Once
	logger     *slog.Logger
	q          *queue.Queue
	schedular  *queue.Schedular
	dbQ        *db.Queries
	wg         sync.WaitGroup
	stop       sync.Once
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewPool(numWorkers int, logger *slog.Logger, q *queue.Queue, schedular *queue.Schedular, dbQ *db.Queries) *Pool {
	return &Pool{
		numWorkers: numWorkers,
		start:      sync.Once{},
		logger:     logger,
		q:          q,
		schedular:  schedular,
		dbQ:        dbQ,
		wg:         sync.WaitGroup{},
		stop:       sync.Once{},
	}
}

func (p *Pool) Start() {
	p.start.Do(func() {
		p.ctx, p.cancel = context.WithCancel(context.Background())
		for i := 1; i <= p.numWorkers; i++ {
			p.wg.Go(func() {
				func(workerNum int) {
					p.logger.Info("worker started", slog.Int("worker_num", workerNum))
				outer:
					for {
						select {
						case <-p.ctx.Done():
							break outer
						default:
						}
						func() {
							defer func() {
								if r := recover(); r != nil {
									p.logger.Error("worker crashed", slog.Int("worker_num", workerNum), slog.Any("error", r))
								}
							}()
							DoWork(p.q, p.schedular, p.dbQ, p.logger, workerNum, p.ctx)
						}()
					}
					p.logger.Info("worker finished", slog.Int("worker_num", workerNum))
				}(i)
			})
		}
	})
}

func (p *Pool) Stop() {
	p.stop.Do(func() {
		p.cancel()
		p.wg.Wait()
	})
}
