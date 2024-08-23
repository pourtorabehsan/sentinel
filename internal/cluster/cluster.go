package cluster

import (
	"log"
	"sync"
)

type Cluster struct {
	mu     sync.RWMutex
	name   string
	config Config
}

func NewCluster(name string, conf Config) (*Cluster, error) {
	cluster := &Cluster{
		name:   name,
		config: conf,
	}

	return cluster, nil
}

func (c *Cluster) Monitor() error {
	log.Printf("monitoring cluster %s", c.name)
	return nil
}
