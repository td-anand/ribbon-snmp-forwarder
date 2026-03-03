// Package snmp provides the SNMP poller that periodically queries a Ribbon SBC
// using SNMP GET-BULK / WALK operations and converts results to TelemetryEvents.
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

// Poller performs periodic SNMP GET-BULK walks against a Ribbon SBC target.
type Poller struct {
	cfg     config.PollConfig
	handler TrapHandler // reuses the same handler type for emitting events
	log     *zap.Logger
}

// NewPoller creates a Poller from the supplied configuration.
func NewPoller(cfg config.PollConfig, handler TrapHandler, log *zap.Logger) *Poller {
	return &Poller{cfg: cfg, handler: handler, log: log}
}

// Start runs the polling loop until ctx is cancelled.
func (p *Poller) Start(ctx context.Context) error {
	if !p.cfg.Enabled {
		p.log.Info("SNMP poller disabled – skipping")
		return nil
	}

	interval := time.Duration(p.cfg.IntervalSeconds) * time.Second
	p.log.Info("starting SNMP poller",
		zap.String("target", p.cfg.Target),
		zap.Duration("interval", interval),
		zap.Strings("oids", p.cfg.OIDs),
	)

	// Run an initial poll immediately on startup.
	p.poll(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("stopping SNMP poller")
			return nil
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

// poll performs a single full poll cycle: it walks every configured OID subtree
// and emits one TelemetryEvent per OID subtree that returns results.
func (p *Poller) poll(ctx context.Context) {
	host, portStr, err := net.SplitHostPort(p.cfg.Target)
	if err != nil {
		// Target may be host-only; default to port 161.
		host = p.cfg.Target
		portStr = "161"
	}

	port := uint16(161)
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		p.log.Warn("invalid poll target port, defaulting to 161",
			zap.String("port_str", portStr), zap.Error(err))
	}

	snmpClient := &gosnmp.GoSNMP{
		Target:             host,
		Port:               port,
		Community:          p.cfg.Community,
		Version:            gosnmp.Version2c,
		Timeout:            time.Duration(p.cfg.TimeoutSeconds) * time.Second,
		Retries:            p.cfg.Retries,
		MaxOids:            gosnmp.MaxOids,
		MaxRepetitions:     50,
		ExponentialTimeout: true,
	}

	if err := snmpClient.Connect(); err != nil {
		p.log.Error("SNMP connect failed", zap.String("target", p.cfg.Target), zap.Error(err))
		return
	}
	defer snmpClient.Conn.Close()

	for _, rootOID := range p.cfg.OIDs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		pdus, err := snmpClient.BulkWalkAll(rootOID)
		if err != nil {
			p.log.Warn("SNMP walk failed",
				zap.String("oid", rootOID),
				zap.String("target", p.cfg.Target),
				zap.Error(err),
			)
			continue
		}

		if len(pdus) == 0 {
			continue
		}

		event := models.TelemetryEvent{
			Timestamp:    time.Now().UTC(),
			Source:       models.SourcePoll,
			AgentAddress: host,
			Community:    p.cfg.Community,
			VarBinds:     convertVarBinds(pdus),
		}

		p.log.Debug("poll cycle completed",
			zap.String("root_oid", rootOID),
			zap.Int("var_binds", len(event.VarBinds)),
		)

		p.handler(ctx, event)
	}
}
