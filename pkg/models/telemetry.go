// Package models defines the telemetry data structures used across the service.
package models

import "time"

// TelemetrySource indicates how a telemetry event was collected.
type TelemetrySource string

const (
	// SourceTrap indicates the event was received as an SNMP trap.
	SourceTrap TelemetrySource = "trap"
	// SourcePoll indicates the event was collected via SNMP polling.
	SourcePoll TelemetrySource = "poll"
)

// VarBind represents a single SNMP variable binding (OID + value pair).
type VarBind struct {
	OID   string `json:"oid"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// TelemetryEvent is the canonical envelope sent to Kafka for every SNMP
// data point, regardless of whether it was collected via trap or poll.
type TelemetryEvent struct {
	// Timestamp is when the event was collected (UTC).
	Timestamp time.Time `json:"timestamp"`
	// Source identifies whether the data came from a trap or a poll cycle.
	Source TelemetrySource `json:"source"`
	// AgentAddress is the IP address of the Ribbon SBC that originated the data.
	AgentAddress string `json:"agent_address"`
	// Community is the SNMP community string used.
	Community string `json:"community"`
	// Enterprise is the enterprise OID (populated for traps).
	Enterprise string `json:"enterprise,omitempty"`
	// TrapType is the generic trap type number (populated for traps).
	TrapType int `json:"trap_type,omitempty"`
	// VarBinds contains the OID/value pairs included in this event.
	VarBinds []VarBind `json:"var_binds"`
}
