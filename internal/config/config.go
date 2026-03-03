// Package config handles loading and validating service configuration from a
// YAML file and/or environment variables via Viper.
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config is the root configuration structure for the ribbon-snmp-forwarder service.
type Config struct {
	Log   LogConfig   `mapstructure:"log"`
	SNMP  SNMPConfig  `mapstructure:"snmp"`
	Kafka KafkaConfig `mapstructure:"kafka"`
}

// LogConfig controls structured logging behaviour.
type LogConfig struct {
	// Level is one of: debug, info, warn, error (default: info).
	Level string `mapstructure:"level"`
	// Format is one of: json, console (default: json).
	Format string `mapstructure:"format"`
}

// SNMPConfig groups all SNMP-related settings.
type SNMPConfig struct {
	// Trap holds settings for the built-in trap listener.
	Trap TrapConfig `mapstructure:"trap"`
	// Poll holds settings for the active polling engine.
	Poll PollConfig `mapstructure:"poll"`
}

// TrapConfig configures the SNMP trap receiver (UDP listener).
type TrapConfig struct {
	// ListenAddress is the host:port to listen on (default: 0.0.0.0:162).
	ListenAddress string `mapstructure:"listen_address"`
	// Community is the expected SNMPv2c community string.
	Community string `mapstructure:"community"`
}

// PollConfig configures the active SNMP poller that queries the Ribbon SBC.
type PollConfig struct {
	// Enabled controls whether the poller is started (default: true).
	Enabled bool `mapstructure:"enabled"`
	// Target is the host:port of the Ribbon SBC to poll (default: :161).
	Target string `mapstructure:"target"`
	// Community is the SNMPv2c community string for polling.
	Community string `mapstructure:"community"`
	// IntervalSeconds is how often (in seconds) a full poll cycle runs (default: 60).
	IntervalSeconds int `mapstructure:"interval_seconds"`
	// OIDs is the list of OIDs/MIB subtrees to walk each poll cycle.
	OIDs []string `mapstructure:"oids"`
	// Retries is the number of SNMP retries on timeout (default: 2).
	Retries int `mapstructure:"retries"`
	// TimeoutSeconds is the per-request SNMP timeout in seconds (default: 5).
	TimeoutSeconds int `mapstructure:"timeout_seconds"`
}

// KafkaConfig holds all Kafka producer settings.
type KafkaConfig struct {
	// Brokers is a list of bootstrap broker addresses (host:port).
	Brokers []string `mapstructure:"brokers"`
	// Topic is the Kafka topic telemetry events are published to.
	Topic string `mapstructure:"topic"`
	// ClientID is the Kafka producer client identifier.
	ClientID string `mapstructure:"client_id"`
}

// Load reads configuration from the given file path.  Environment variables
// prefixed with SNMPFWD_ override any value in the file.
func Load(path string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("snmp.trap.listen_address", "0.0.0.0:162")
	v.SetDefault("snmp.trap.community", "public")
	v.SetDefault("snmp.poll.enabled", true)
	v.SetDefault("snmp.poll.target", "127.0.0.1:161")
	v.SetDefault("snmp.poll.community", "public")
	v.SetDefault("snmp.poll.interval_seconds", 60)
	v.SetDefault("snmp.poll.retries", 2)
	v.SetDefault("snmp.poll.timeout_seconds", 5)
	v.SetDefault("snmp.poll.oids", []string{
		"1.3.6.1.2.1.1",   // System MIB
		"1.3.6.1.2.1.2",   // Interfaces MIB
		"1.3.6.1.4.1.2879", // Ribbon/GENBAND enterprise OIDs
	})
	v.SetDefault("kafka.topic", "snmp-telemetry")
	v.SetDefault("kafka.client_id", "ribbon-snmp-forwarder")

	// File-based config
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	// Environment variable overrides: SNMPFWD_KAFKA_BROKERS, etc.
	v.SetEnvPrefix("SNMPFWD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// validate checks that mandatory fields are present and values are sane.
func validate(cfg *Config) error {
	if len(cfg.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers must contain at least one broker address")
	}
	if cfg.Kafka.Topic == "" {
		return fmt.Errorf("kafka.topic must not be empty")
	}
	if cfg.SNMP.Trap.ListenAddress == "" {
		return fmt.Errorf("snmp.trap.listen_address must not be empty")
	}
	if cfg.SNMP.Poll.Enabled && cfg.SNMP.Poll.Target == "" {
		return fmt.Errorf("snmp.poll.target must not be empty when polling is enabled")
	}
	if cfg.SNMP.Poll.IntervalSeconds <= 0 {
		return fmt.Errorf("snmp.poll.interval_seconds must be > 0")
	}
	return nil
}
