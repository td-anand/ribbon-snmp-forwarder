package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/td-anand/ribbon-snmp-forwarder/internal/config"
)

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("creating temp config file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestLoad_ValidConfig(t *testing.T) {
	yaml := `
log:
  level: debug
  format: console
snmp:
  trap:
    listen_address: "0.0.0.0:1162"
    community: "testcommunity"
  poll:
    enabled: true
    target: "10.0.0.1:161"
    community: "testcommunity"
    interval_seconds: 30
    retries: 1
    timeout_seconds: 3
    oids:
      - "1.3.6.1.2.1.1"
kafka:
  brokers:
    - "localhost:9092"
  topic: "test-topic"
  client_id: "test-client"
`
	path := writeYAML(t, yaml)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Log.Level != "debug" {
		t.Errorf("log.level = %q; want %q", cfg.Log.Level, "debug")
	}
	if cfg.SNMP.Trap.ListenAddress != "0.0.0.0:1162" {
		t.Errorf("snmp.trap.listen_address = %q; want %q", cfg.SNMP.Trap.ListenAddress, "0.0.0.0:1162")
	}
	if cfg.SNMP.Poll.Target != "10.0.0.1:161" {
		t.Errorf("snmp.poll.target = %q; want %q", cfg.SNMP.Poll.Target, "10.0.0.1:161")
	}
	if cfg.SNMP.Poll.IntervalSeconds != 30 {
		t.Errorf("snmp.poll.interval_seconds = %d; want 30", cfg.SNMP.Poll.IntervalSeconds)
	}
	if cfg.Kafka.Topic != "test-topic" {
		t.Errorf("kafka.topic = %q; want %q", cfg.Kafka.Topic, "test-topic")
	}
	if len(cfg.Kafka.Brokers) != 1 || cfg.Kafka.Brokers[0] != "localhost:9092" {
		t.Errorf("kafka.brokers = %v; want [localhost:9092]", cfg.Kafka.Brokers)
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Minimal config – only mandatory fields.
	yaml := `
snmp:
  trap:
    listen_address: "0.0.0.0:1162"
  poll:
    target: "10.0.0.1:161"
kafka:
  brokers:
    - "broker:9092"
  topic: "telemetry"
`
	path := writeYAML(t, yaml)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Log.Level != "info" {
		t.Errorf("default log.level = %q; want %q", cfg.Log.Level, "info")
	}
	if cfg.Log.Format != "json" {
		t.Errorf("default log.format = %q; want %q", cfg.Log.Format, "json")
	}
	if cfg.SNMP.Poll.IntervalSeconds != 60 {
		t.Errorf("default poll.interval_seconds = %d; want 60", cfg.SNMP.Poll.IntervalSeconds)
	}
	if cfg.SNMP.Poll.Retries != 2 {
		t.Errorf("default poll.retries = %d; want 2", cfg.SNMP.Poll.Retries)
	}
}

func TestLoad_MissingBrokers(t *testing.T) {
	yaml := `
snmp:
  trap:
    listen_address: "0.0.0.0:1162"
  poll:
    target: "10.0.0.1:161"
kafka:
  topic: "telemetry"
`
	path := writeYAML(t, yaml)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for missing kafka.brokers, got nil")
	}
}

func TestLoad_ZeroIntervalSeconds(t *testing.T) {
	yaml := `
snmp:
  trap:
    listen_address: "0.0.0.0:1162"
  poll:
    target: "10.0.0.1:161"
    interval_seconds: 0
kafka:
  brokers:
    - "broker:9092"
  topic: "telemetry"
`
	path := writeYAML(t, yaml)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for interval_seconds=0, got nil")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing config file, got nil")
	}
}
