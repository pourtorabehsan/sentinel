package instance

import "time"

type Config struct {
	MonitorCredentials      Credentials
	ReplicationCredentials  Credentials
	UseMasterInfoRepository bool
	HealthCheckInterval     time.Duration
}

