package monitor

import "time"

type Config struct {
	MonitorUsername     string
	MonitorPassword     string
	ReplicationUsername string
	ReplicationPassword string
	CheckInterval       time.Duration
}

type instanceConfig struct {
	monitorUsername     string
	monitorPassword     string
	replicationUsername string
	replicationPassword string
	checkInterval       time.Duration
}

func (c Config) instanceConfig() instanceConfig {
	return instanceConfig{
		monitorUsername:     c.MonitorUsername,
		monitorPassword:     c.MonitorPassword,
		replicationUsername: c.ReplicationUsername,
		replicationPassword: c.ReplicationPassword,
		checkInterval:       c.CheckInterval,
	}
}
