package main

import (
	"fmt"
	"log"
	"time"

	"github.com/pourtorabehsan/sentinel/internal/cluster"
)

func main() {
	config := cluster.Config{
		MonitorUsername:     "monitor",
		MonitorPassword:     "monitor",
		ReplicationUsername: "repl",
		ReplicationPassword: "repl",
		CheckInterval:       1 * time.Second,
	}
	cluster, err := cluster.NewCluster("my-cluster", config)
	if err != nil {
		log.Fatalf("failed to create cluster: %v", err)
	}

	err = cluster.Monitor()
	if err != nil {
		fmt.Printf("failed to monitor cluster: %v", err)
	}

	fmt.Printf("Goodbye!")
}
