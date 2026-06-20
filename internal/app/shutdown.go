package app

import (
	"sync"
	"time"
)

const (
	_workerShutdownPeriod = 20 * time.Second
	_shutdownHardPeriod   = 3 * time.Second
	_shutdownServerPeriod = 15 * time.Second
)

// Shutdown gracefully shuts down the app
func (a *App) Shutdown() {
	a.logger.Info("Received shutdown signal, initiating shut down.")
	var wg sync.WaitGroup
	wg.Go(func() {
		a.workerPool.Stop(_workerShutdownPeriod)
	})
	wg.Go(func() {
		if err := a.server.Shutdown(_shutdownServerPeriod); err != nil {
			a.logger.Error("Failed to wait for ongoing requests to finish, waiting for forced cancellation.")
			time.Sleep(_shutdownHardPeriod)
		}
		a.logger.Info("server shut down successfully")
	})
	wg.Wait()
	a.dbPool.Close()
	a.logger.Info("shutdown gracefully!")
}
