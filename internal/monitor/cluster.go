package monitor

import (
	"context"
	"log"
	"sync"
)

type Cluster struct {
	mu        sync.RWMutex
	name      string
	instances []*Instance
	config    Config
}

func NewCluster(name string, primaryEndpoint Endpoint, conf Config) (*Cluster, error) {
	cluster := &Cluster{
		name:   name,
		config: conf,
	}

	primary, err := openInstance(primaryEndpoint, conf.instanceConfig())
	if err != nil {
		return nil, err
	}

	cluster.addInstance(primary)
	return cluster, nil
}

func (c *Cluster) addInstance(i *Instance) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.instances = append(c.instances, i)
}

func (c *Cluster) getInstances() []*Instance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.instances
}

func (c *Cluster) Monitor(ctx context.Context) error {
	log.Printf("monitoring cluster %s", c.name)
	return nil
}
