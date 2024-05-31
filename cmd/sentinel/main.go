package main

import (
	"context"
	"fmt"
	"time"

	"github.com/pourtorabehsan/sentinel/internal/monitor"
)

func main() {
	config := monitor.Config{
		MonitorUsername:     "root",
		MonitorPassword:     "sa",
		ReplicationUsername: "root",
		ReplicationPassword: "sa",
		CheckInterval:       10 * time.Second,
	}
	cluster := monitor.NewCluster("my-cluster", monitor.NewInstance("127.0.0.1", 8080), config)
	err := cluster.Monitor(context.Background())
	if err != nil {
		fmt.Printf("failed to monitor cluster: %v", err)
	}

	fmt.Printf("Goodbye!")
}
