package monitor

import "time"

type Config struct {
	MonitorUsername     string
	MonitorPassword     string
	ReplicationUsername string
	ReplicationPassword string
	CheckInterval       time.Duration
}
