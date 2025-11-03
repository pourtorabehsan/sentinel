package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	version = "0.1.0-dev"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "sentinel",
	Short: "Sentinel - MySQL High Availability and Failover Tool",
	Long: `Sentinel is a MySQL high availability and automated failover tool.
It monitors MySQL replication topologies and performs automatic failover
when the primary fails, ensuring minimal downtime.`,
	Version: version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml",
		"config file path")
}
