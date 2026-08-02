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

	// TODO: start worker pool (internal/worker) — fixed-size pool of
	// goroutines pulling jobs off jobQueue.Jobs() and processing exports.

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

	// TODO: cancel worker pool context, let in-flight jobs finish

	log.Println("[main] graceful shutdown complete")
}
