package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/pourtorabehsan/sentinel/internal/config"
	"github.com/pourtorabehsan/sentinel/internal/logger"
)

// serveCmd represents the serve command (daemon mode).
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start Sentinel server (daemon mode)",
	Long: `Start Sentinel in server/daemon mode.

In this mode, Sentinel continuously monitors registered MySQL clusters,
detects failures, and performs automatic failover when needed.

This is the main operational mode for production use.`,
	RunE: runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.LoadFromFile(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	level := logger.ParseLevel(cfg.LogLevel)
	log := logger.New(level)

	log.Info("starting sentinel server",
		"version", version,
		"config", cfgFile,
		"mode", cfg.Mode,
	)

	// Log configuration (sanitized)
	log.Info("configuration loaded",
		"mode", cfg.Mode,
		"state_store_type", cfg.StateStore.Type,
		"state_store_dsn", cfg.StateStore.SanitizedDSN(),
		"leader_election_backend", cfg.LeaderElection.Backend,
		"api_enabled", cfg.API.Enabled,
		"api_port", cfg.API.Port,
		"monitor_enabled", cfg.Monitor.Enabled,
		"monitor_interval", cfg.Monitor.IntervalSeconds,
	)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Info("sentinel server initialized, waiting for signals")
	log.Info("NOTE: Full server functionality will be implemented in Phase 2+")

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
	// - Shut down API server

	log.Info("shutdown complete")
	return nil
}
