// Package main is the entry point for the Sentinel application.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pourtorabehsan/sentinel/internal/config"
	"github.com/pourtorabehsan/sentinel/internal/logger"
)

const version = "0.1.0-dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse command-line flags
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	logLevel := flag.String("log-level", "info", "log level (debug, info, warn, error)")
	showVersion := flag.Bool("version", false, "show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Sentinel version %s\n", version)
		return nil
	}

	// Initialize logger
	level := logger.ParseLevel(*logLevel)
	log := logger.New(level)

	log.Info("starting sentinel",
		"version", version,
		"config_path", *configPath,
		"log_level", logLevel,
	)

	// Load configuration
	cfg, err := config.LoadFromFile(*configPath)
	if err != nil {
		log.Error("failed to load configuration", "error", err)
		return fmt.Errorf("configuration error: %w", err)
	}

	// Override log level from config if specified
	if cfg.LogLevel != "" {
		level = logger.ParseLevel(cfg.LogLevel)
		log = logger.New(level)
		log.Info("log level updated from config", "level", cfg.LogLevel)
	}

	// Log configuration (sanitized)
	log.Info("configuration loaded successfully",
		"mode", cfg.Mode,
		"state_store_type", cfg.StateStore.Type,
		"state_store_dsn", cfg.StateStore.SanitizedDSN(),
		"leader_election_backend", cfg.LeaderElection.Backend,
		"leader_election_endpoints", cfg.LeaderElection.Endpoints,
		"groups_count", len(cfg.Groups),
	)

	// Log details about each group and cluster
	for _, group := range cfg.Groups {
		groupLog := logger.WithGroup(log, group.ID)
		groupLog.Info("group configured",
			"clusters_count", len(group.Clusters),
		)

		for _, cluster := range group.Clusters {
			clusterLog := logger.WithContext(groupLog, group.ID, cluster.ID, "")
			clusterLog.Info("cluster configured",
				"seed_nodes_count", len(cluster.SeedNodes),
			)

			for _, node := range cluster.SeedNodes {
				nodeID := fmt.Sprintf("%s:%d", node.Host, node.Port)
				nodeLog := logger.WithNode(clusterLog, nodeID)
				nodeLog.Debug("seed node configured",
					"host", node.Host,
					"port", node.Port,
				)
			}
		}
	}

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Info("sentinel initialized successfully, waiting for signals")

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		log.Info("received shutdown signal", "signal", sig)
	case <-ctx.Done():
		log.Info("context cancelled")
	}

	log.Info("shutting down gracefully")

	// In future phases, this is where we would:
	// - Stop monitoring goroutines
	// - Release leader election lock
	// - Close database connections
	// - Flush any pending state updates

	log.Info("shutdown complete")
	return nil
}
