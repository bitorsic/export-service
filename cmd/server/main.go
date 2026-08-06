package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bitorsic/export-service/internal/api"
	"github.com/bitorsic/export-service/internal/config"
	"github.com/bitorsic/export-service/internal/db"
	"github.com/bitorsic/export-service/internal/jobs"
	"github.com/bitorsic/export-service/internal/queue"
	"github.com/bitorsic/export-service/internal/worker"
)

func main() {
	ctx := context.Background()

	cfg := config.Load()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[main] failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("[main] connected to database")

	jobStore := jobs.NewStore(pool)
	jobQueue := queue.New(20) // bounded capacity

	handler := api.NewHandler(jobStore, jobQueue)
	router := api.NewRouter(handler)

	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		log.Printf("[main] listening on :%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[main] HTTP server failed: %v", err)
		}
	}()

	// workerCtx is separate from the top-level ctx so we can cancel it
	// independently during shutdown — this is the signal that tells
	// workers "stop picking up new jobs."
	workerCtx, cancelWorkers := context.WithCancel(ctx)

	workerPool := worker.NewPool(5, jobQueue, jobStore, pool)
	var workersDone = make(chan struct{})
	go func() {
		workerPool.Start(workerCtx)
		close(workersDone)
	}()

	// Wait for shutdown signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("[main] shutdown signal received")

	// Stop accepting new HTTP requests, but let in-flight ones finish
	// (bounded by the context timeout) before exiting.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[main] HTTP server shutdown error: %v", err)
	}

	// Signal workers to stop picking up NEW jobs, then wait for any
	// in-flight export to finish (runExport checks ctx.Err() to avoid
	// running forever) before we exit.
	cancelWorkers()
	<-workersDone
	log.Println("[main] worker pool drained")

	log.Println("[main] graceful shutdown complete")
}
