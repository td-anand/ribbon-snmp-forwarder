package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/td-anand/ribbon-snmp-forwarder/pkg/models"
)

func TestTelemetryEvent_JSONRoundTrip(t *testing.T) {
	original := models.TelemetryEvent{
		Timestamp:    time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC),
		Source:       models.SourceTrap,
		AgentAddress: "192.168.1.1",
		Community:    "public",
		Enterprise:   "1.3.6.1.4.1.2879",
		TrapType:     6,
		VarBinds: []models.VarBind{
			{OID: "1.3.6.1.2.1.1.1.0", Type: "OctetString", Value: "Ribbon SBC"},
			{OID: "1.3.6.1.2.1.1.3.0", Type: "TimeTicks", Value: "12345"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded models.TelemetryEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Source != original.Source {
		t.Errorf("Source = %q; want %q", decoded.Source, original.Source)
	}
	if decoded.AgentAddress != original.AgentAddress {
		t.Errorf("AgentAddress = %q; want %q", decoded.AgentAddress, original.AgentAddress)
	}
	if decoded.Enterprise != original.Enterprise {
		t.Errorf("Enterprise = %q; want %q", decoded.Enterprise, original.Enterprise)
	}
	if decoded.TrapType != original.TrapType {
		t.Errorf("TrapType = %d; want %d", decoded.TrapType, original.TrapType)
	}
	if len(decoded.VarBinds) != len(original.VarBinds) {
		t.Fatalf("VarBinds length = %d; want %d", len(decoded.VarBinds), len(original.VarBinds))
	}
	if decoded.VarBinds[0].OID != original.VarBinds[0].OID {
		t.Errorf("VarBinds[0].OID = %q; want %q", decoded.VarBinds[0].OID, original.VarBinds[0].OID)
	}
}

func TestTelemetryEvent_PollSource(t *testing.T) {
	event := models.TelemetryEvent{
		Timestamp:    time.Now().UTC(),
		Source:       models.SourcePoll,
		AgentAddress: "10.0.0.1",
		Community:    "public",
		VarBinds:     []models.VarBind{},
	}

	if event.Source != models.SourcePoll {
		t.Errorf("Source = %q; want %q", event.Source, models.SourcePoll)
	}
	if event.Enterprise != "" {
		t.Errorf("Enterprise should be empty for poll events, got %q", event.Enterprise)
	}
}

func TestVarBind_Fields(t *testing.T) {
	vb := models.VarBind{
		OID:   "1.3.6.1.2.1.1.1.0",
		Type:  "OctetString",
		Value: "test value",
	}
	if vb.OID == "" || vb.Type == "" || vb.Value == "" {
		t.Errorf("VarBind fields must not be empty: %+v", vb)
	}
}
