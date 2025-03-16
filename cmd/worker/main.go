package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"laile/internal/config"
	"laile/internal/database"
	"laile/internal/event"
	"laile/internal/log"
)

func main() {
	// Initialize logger
	log.InitLogger()
	log.Logger.Info("Starting webhook worker")

	// Load configuration
	conf, err := config.LoadMainConfig()
	if err != nil {
		log.Logger.Error("Failed to load configuration", "error", err)
		panic(fmt.Sprintf("cannot load config: %s", err))
	}

	// Initialize database connection with retry logic
	var db database.Service
	maxRetries := 5
	retryDelay := 3 * time.Second

	for i := range maxRetries {
		db, err = database.New(&conf.Database)
		if err == nil {
			break
		}

		log.Logger.Error("Failed to initialize database connection", "error", err, "attempt", i+1, "max_retries", maxRetries)

		if i < maxRetries-1 {
			log.Logger.Info("Retrying database connection", "retry_delay", retryDelay)
			time.Sleep(retryDelay)
			// Exponential backoff
			retryDelay *= 2
		} else {
			panic(fmt.Sprintf("failed to connect to database after %d attempts: %s", maxRetries, err))
		}
	}

	// Ensure database connection is closed when the program exits
	defer db.Close()

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Run the event processor in a loop to handle restarts
	for {
		select {
		case <-ctx.Done():
			log.Logger.Info("Shutting down webhook worker")
			return
		default:
			log.Logger.Info("Starting webhook event processor")

			// If ProcessEvents returns, it means there was a critical error
			err = event.ProcessEvents(ctx, db, conf)

			// Log the error and restart the process
			log.Logger.Error("Event processor failed, restarting process", "error", err)

			// If context is cancelled, exit instead of restarting
			if ctx.Err() != nil {
				return
			}

			// Panic to allow Docker to restart the process if needed
			panic(fmt.Sprintf("event processor failed: %s", err))
		}
	}
}
