// Package snmp provides the SNMP trap receiver for the ribbon-snmp-forwarder
// service.  It listens on a UDP port for SNMPv1 and SNMPv2c trap PDUs sent
// by Ribbon SBC devices and converts them to TelemetryEvents.
package snmp

import (
	"context"
	"fmt"
	"net"
	"time"

	gosnmp "github.com/gosnmp/gosnmp"
	"go.uber.org/zap"

	"github.com/td-anand/ribbon-snmp-forwarder/internal/config"
	"github.com/td-anand/ribbon-snmp-forwarder/pkg/models"
)

// TrapHandler is called for every valid trap received.
type TrapHandler func(ctx context.Context, event models.TelemetryEvent)

// TrapReceiver listens for incoming SNMP traps.
type TrapReceiver struct {
	cfg     config.TrapConfig
	handler TrapHandler
	log     *zap.Logger
	tl      *gosnmp.TrapListener
}

// NewTrapReceiver creates a TrapReceiver that will deliver events to handler.
func NewTrapReceiver(cfg config.TrapConfig, handler TrapHandler, log *zap.Logger) (*TrapReceiver, error) {
	tr := &TrapReceiver{
		cfg:     cfg,
		handler: handler,
		log:     log,
	}

	tl := gosnmp.NewTrapListener()
	tl.Params = gosnmp.Default
	tl.Params.Community = cfg.Community
	tl.OnNewTrap = tr.onTrap

	tr.tl = tl
	return tr, nil
}

// Start begins listening for traps.  It blocks until ctx is cancelled or an
// unrecoverable error occurs.
func (r *TrapReceiver) Start(ctx context.Context) error {
	r.log.Info("starting SNMP trap listener", zap.String("address", r.cfg.ListenAddress))

	errCh := make(chan error, 1)
	go func() {
		if err := r.tl.Listen(r.cfg.ListenAddress); err != nil {
			errCh <- fmt.Errorf("trap listener exited: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		r.log.Info("stopping SNMP trap listener")
		r.tl.Close()
		return nil
	case err := <-errCh:
		return err
	}
}

// onTrap is called by the gosnmp library for each received trap PDU.
func (r *TrapReceiver) onTrap(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
	agentAddr := addr.IP.String()
	if packet.AgentAddress != "" {
		agentAddr = packet.AgentAddress
	}

	event := models.TelemetryEvent{
		Timestamp:    time.Now().UTC(),
		Source:       models.SourceTrap,
		AgentAddress: agentAddr,
		Community:    packet.Community,
		VarBinds:     convertVarBinds(packet.Variables),
	}

	if packet.PDUType == gosnmp.Trap {
		event.Enterprise = packet.Enterprise
		event.TrapType = int(packet.GenericTrap)
	}

	r.log.Debug("received SNMP trap",
		zap.String("agent", agentAddr),
		zap.Int("var_binds", len(event.VarBinds)),
	)

	r.handler(context.Background(), event)
}

// convertVarBinds translates gosnmp.SnmpPDU slice to the canonical VarBind model.
func convertVarBinds(pdus []gosnmp.SnmpPDU) []models.VarBind {
	vbs := make([]models.VarBind, 0, len(pdus))
	for _, pdu := range pdus {
		vbs = append(vbs, models.VarBind{
			OID:   pdu.Name,
			Type:  pduTypeName(pdu.Type),
			Value: fmt.Sprintf("%v", pdu.Value),
		})
	}
	return vbs
}

// pduTypeName returns a human-readable name for a gosnmp Asn1BER type.
func pduTypeName(t gosnmp.Asn1BER) string {
	switch t {
	case gosnmp.Integer:
		return "Integer"
	case gosnmp.OctetString:
		return "OctetString"
	case gosnmp.ObjectIdentifier:
		return "OID"
	case gosnmp.IPAddress:
		return "IPAddress"
	case gosnmp.Counter32:
		return "Counter32"
	case gosnmp.Gauge32:
		return "Gauge32"
	case gosnmp.TimeTicks:
		return "TimeTicks"
	case gosnmp.Counter64:
		return "Counter64"
	case gosnmp.NoSuchObject:
		return "NoSuchObject"
	case gosnmp.NoSuchInstance:
		return "NoSuchInstance"
	default:
		return fmt.Sprintf("0x%02X", int(t))
	}
}
