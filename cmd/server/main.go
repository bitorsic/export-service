package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bitorsic/export-service/internal/config"
	"github.com/bitorsic/export-service/internal/db"
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

	// TODO: start HTTP API (internal/api) — accepts export requests,
	// writes jobs to Postgres, pushes onto the bounded job channel.
	//
	// TODO: start worker pool (internal/worker) — fixed-size pool of
	// goroutines pulling jobs off that channel and processing exports.

	// Wait for shutdown signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("[main] shutdown signal received")

	// TODO: cancel worker pool context, let in-flight jobs finish
	// TODO: shut down HTTP server with a timeout

	log.Println("[main] graceful shutdown complete")
}
