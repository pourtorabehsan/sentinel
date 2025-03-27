package mysql

import (
	"sync"
	"time"
)

// Topology represents a MySQL replication topology
type Topology struct {
	mu sync.RWMutex

	// Master instance
	master *Instance

	// Slave instances
	slaves map[string]*Instance

	// Last topology update
	lastRefresh time.Time
}

// NewTopology creates a new MySQL topology
func NewTopology() *Topology {
	return &Topology{
		slaves: make(map[string]*Instance),
	}
}

// SetMaster updates the master instance
func (t *Topology) SetMaster(instance *Instance) {
	t.mu.Lock()
	defer t.mu.Unlock()

	instance.Role = RoleMaster
	t.master = instance
}

// AddSlave adds or updates a slave instance
func (t *Topology) AddSlave(instance *Instance) {
	t.mu.Lock()
	defer t.mu.Unlock()

	instance.Role = RoleSlave
	t.slaves[instance.Key()] = instance
}

// GetMaster returns the current master instance
func (t *Topology) GetMaster() *Instance {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.master
}

// GetSlaves returns all slave instances
func (t *Topology) GetSlaves() []*Instance {
	t.mu.RLock()
	defer t.mu.RUnlock()

	slaves := make([]*Instance, 0, len(t.slaves))
	for _, slave := range t.slaves {
		slaves = append(slaves, slave)
	}
	return slaves
}

// GetInstance returns an instance by its key
func (t *Topology) GetInstance(key string) *Instance {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.master != nil && t.master.Key() == key {
		return t.master
	}
	return t.slaves[key]
}

// RemoveInstance removes an instance from the topology
func (t *Topology) RemoveInstance(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.master != nil && t.master.Key() == key {
		t.master = nil
	}
	delete(t.slaves, key)
}
