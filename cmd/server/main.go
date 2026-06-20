// Entry point of the application
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/Arush71/jobqueue/internal/app"
)

func main() {
	// Init App
	app, err := app.StartApp()
	if err != nil {
		log.Fatal("error starting the app ", err)
		return
	}
	// Recover lost state
	if err := app.Recover(); err != nil {
		log.Fatal("error recovering jobs", err)
		return
	}
	// Waiting on signal
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-rootCtx.Done()
	stop()
	app.Shutdown()
}
