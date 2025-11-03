package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pourtorabehsan/sentinel/internal/config"
	"github.com/pourtorabehsan/sentinel/internal/discovery"
	"github.com/pourtorabehsan/sentinel/internal/logger"
	"github.com/pourtorabehsan/sentinel/internal/store"
	"github.com/pourtorabehsan/sentinel/internal/types"
)

var (
	discoverClusterID string
	discoverGroupID   string
	discoverSeed      string
	discoverUser      string
	discoverPassword  string
)

// discoverCmd represents the discover command.
var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover MySQL replication topology",
	Long: `Discover the complete MySQL replication topology starting from a seed node.

The discover command connects to a MySQL instance (seed node) and crawls
the replication topology by following replication links upstream and downstream.
It identifies the primary, all replicas, and their relationships.

The discovered topology is stored in the database and can be viewed or managed
using other Sentinel commands.

Example:
  sentinel discover --cluster-id=prod-mysql-1 --seed=localhost:3306 --user=root --password=secret`,
	RunE: runDiscover,
}

func init() {
	rootCmd.AddCommand(discoverCmd)

	// Required flags
	discoverCmd.Flags().StringVar(&discoverClusterID, "cluster-id", "",
		"Cluster ID (required)")
	discoverCmd.MarkFlagRequired("cluster-id")

	discoverCmd.Flags().StringVar(&discoverSeed, "seed", "",
		"Seed node in format host:port (required)")
	discoverCmd.MarkFlagRequired("seed")

	discoverCmd.Flags().StringVar(&discoverUser, "user", "root",
		"MySQL username")
	discoverCmd.Flags().StringVar(&discoverPassword, "password", "",
		"MySQL password")

	// Optional flags
	discoverCmd.Flags().StringVar(&discoverGroupID, "group-id", "default",
		"Group ID (default: 'default')")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	startTime := time.Now()

	// Parse seed address
	parts := strings.Split(discoverSeed, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid seed format: expected host:port, got: %s", discoverSeed)
	}
	seedHost := parts[0]
	var seedPort int
	if _, err := fmt.Sscanf(parts[1], "%d", &seedPort); err != nil {
		return fmt.Errorf("invalid port in seed: %s", parts[1])
	}

	// Load config
	cfg, err := config.LoadFromFile(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	level := logger.ParseLevel(cfg.LogLevel)
	log := logger.New(level)

	log.Info("starting discovery",
		"cluster_id", discoverClusterID,
		"group_id", discoverGroupID,
		"seed", discoverSeed,
	)

	// Initialize store
	var st store.Store
	switch strings.ToLower(cfg.StateStore.Type) {
	case "sqlite":
		st, err = store.NewSQLiteStore(cfg.StateStore.DSN)
		if err != nil {
			return fmt.Errorf("failed to create SQLite store: %w", err)
		}
		defer st.Close()
	default:
		return fmt.Errorf("unsupported store type: %s (only sqlite is supported in Phase 1)",
			cfg.StateStore.Type)
	}

	ctx := context.Background()

	// Ensure group exists
	group, err := st.GetGroup(ctx, discoverGroupID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("failed to get group: %w", err)
		}
		// Group doesn't exist, create it
		log.Info("creating group", "group_id", discoverGroupID)
		group = &types.Group{
			ID:          discoverGroupID,
			Name:        discoverGroupID,
			Description: fmt.Sprintf("Auto-created group for %s", discoverGroupID),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := st.CreateGroup(ctx, group); err != nil {
			return fmt.Errorf("failed to create group: %w", err)
		}
	}

	// Check if cluster exists, create if not
	cluster, err := st.GetCluster(ctx, discoverClusterID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("failed to get cluster: %w", err)
		}
		// Cluster doesn't exist, create it
		log.Info("creating cluster", "cluster_id", discoverClusterID)
		cluster = &types.Cluster{
			ID:          discoverClusterID,
			GroupID:     discoverGroupID,
			Name:        discoverClusterID,
			Description: fmt.Sprintf("Auto-created cluster for %s", discoverClusterID),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := st.CreateCluster(ctx, cluster); err != nil {
			return fmt.Errorf("failed to create cluster: %w", err)
		}
	}

	// Create discovery manager
	discoveryMgr := discovery.NewManager(st, log)

	// Discover topology
	log.Info("discovering topology from seed", "seed", discoverSeed)
	topology, err := discoveryMgr.DiscoverFromSeed(ctx, discoverClusterID,
		seedHost, seedPort, discoverUser, discoverPassword)
	if err != nil {
		return fmt.Errorf("failed to discover topology: %w", err)
	}

	// Save topology to store
	log.Info("saving topology to database")
	if err := discoveryMgr.SaveTopology(ctx, topology); err != nil {
		return fmt.Errorf("failed to save topology: %w", err)
	}

	// Print topology tree
	fmt.Println()
	fmt.Printf("Cluster: %s\n", discoverClusterID)
	if topology.Primary != nil {
		printTopologyTree(topology.Tree, "")
	} else {
		fmt.Println("  No primary found!")
		fmt.Println("  Nodes:")
		for _, node := range topology.Nodes {
			fmt.Printf("    - %s (%s:%d) [%s]\n",
				node.ID[:8], node.Host, node.Port, node.Role.String())
		}
	}
	fmt.Println()

	elapsed := time.Since(startTime)
	fmt.Printf("Discovered %d node(s) in %.2fs\n", len(topology.Nodes), elapsed.Seconds())

	return nil
}

// printTopologyTree recursively prints the topology tree.
func printTopologyTree(node *types.TopologyNode, prefix string) {
	if node == nil {
		return
	}

	// Determine node symbol based on role
	var symbol string
	switch node.Node.Role {
	case types.RolePrimary:
		symbol = "Primary"
	case types.RoleReplica:
		symbol = "Replica"
	case types.RoleIntermediate:
		symbol = "Intermediate"
	default:
		symbol = "Unknown"
	}

	// Print current node
	fmt.Printf("%s  %s: %s (%s:%d) [%s %s]\n",
		prefix,
		symbol,
		node.Node.ID[:8],
		node.Node.Host,
		node.Node.Port,
		node.Node.MySQLVersion,
		formatServerID(node.Node.ServerID),
	)

	// Print children
	for i, child := range node.Children {
		var childPrefix string
		if i == len(node.Children)-1 {
			childPrefix = prefix + "    └─"
		} else {
			childPrefix = prefix + "    ├─"
		}
		printTopologyTree(child, childPrefix)
	}
}

// formatServerID formats a server ID for display.
func formatServerID(serverID uint32) string {
	if serverID == 0 {
		return ""
	}
	return fmt.Sprintf("server_id=%d", serverID)
}
