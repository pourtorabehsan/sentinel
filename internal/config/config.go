package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Monitor   MonitorConfig   `mapstructure:"monitor"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	Failover  FailoverConfig  `mapstructure:"failover"`
}

type MonitorConfig struct {
	Interval          time.Duration `mapstructure:"interval"`
	HealthCheckRetry  int          `mapstructure:"health_check_retry"`
	HealthCheckDelay  time.Duration `mapstructure:"health_check_delay"`
}

type HTTPConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

type FailoverConfig struct {
	Automatic       bool          `mapstructure:"automatic"`
	MaxFailureTime  time.Duration `mapstructure:"max_failure_time"`
	MinReplicaLag   time.Duration `mapstructure:"min_replica_lag"`
}

// Load reads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Read config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/sentinel/")
	}

	// Read environment variables
	v.SetEnvPrefix("SENTINEL")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	config := &Config{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return config, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("monitor.interval", "10s")
	v.SetDefault("monitor.health_check_retry", 3)
	v.SetDefault("monitor.health_check_delay", "5s")
	v.SetDefault("http.port", 8080)
	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("failover.automatic", true)
	v.SetDefault("failover.max_failure_time", "30s")
	v.SetDefault("failover.min_replica_lag", "10s")
}
