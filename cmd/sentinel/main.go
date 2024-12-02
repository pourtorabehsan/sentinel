package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pourtorabehsan/sentinel/internal/api"
	"github.com/pourtorabehsan/sentinel/internal/config"
	"github.com/pourtorabehsan/sentinel/internal/monitor"
	"github.com/pourtorabehsan/sentinel/internal/storage/memory"
	"github.com/pourtorabehsan/sentinel/internal/failover"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Parse command line flags
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize storage
	storage := memory.NewMemoryStorage()

	// Initialize monitor
	mon := monitor.New(
		storage,
		cfg.Monitor.Interval,
		cfg.Monitor.HealthCheckRetry,
		cfg.Monitor.HealthCheckDelay,
	)

	// Initialize failover orchestrator
	orchestrator := failover.NewOrchestrator(
		storage,
		cfg.Failover.MaxFailureTime,
		cfg.Failover.MinReplicaLag,
		cfg.Failover.Automatic,
	)

	// Initialize API handlers
	handler := api.NewHandler(storage)
	router := api.NewRouter(handler)

	// Create HTTP server
	server := &http.Server{
				Addr:    fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
				Handler: router,
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start monitoring
	go mon.Start(ctx)

	// Start failover monitoring
	go func() {
		ticker := time.NewTicker(cfg.Monitor.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := orchestrator.CheckAndFailover(ctx); err != nil {
					log.Printf("Failover check error: %v", err)
				}
			}
		}
	}()

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on %s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down gracefully...")

	// Stop monitoring and shutdown server
	cancel()
	if err := server.Shutdown(context.Background()); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
}
