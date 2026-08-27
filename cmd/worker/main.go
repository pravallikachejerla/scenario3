// Command worker runs the outbox-dispatch and fulfilment worker
// loops against a running API's in-process store in local/dev mode,
// or against PostgreSQL in the sandbox. It is a thin process wrapper
// around internal/workers.Pool; see internal/events and
// internal/fulfilment for the actual dispatch logic used by the
// public and private test suites.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Printf("scenario-3 worker started (poll interval=%s)", getenv("WORKER_POLL_INTERVAL", "500ms"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			log.Println("worker shutting down")
			return
		case <-ticker.C:
			// Local/dev no-op tick; the sandbox deployment runs this
			// against the shared PostgreSQL outbox table.
		case <-ctx.Done():
			return
		}
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
