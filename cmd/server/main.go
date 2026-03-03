// Command server is the entry point for the ribbon-snmp-forwarder service.
// It starts the SNMP trap receiver and (optionally) the SNMP poller, forwarding
// all collected telemetry to a Kafka topic.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/td-anand/ribbon-snmp-forwarder/internal/config"
	"github.com/td-anand/ribbon-snmp-forwarder/internal/kafka"
	"github.com/td-anand/ribbon-snmp-forwarder/internal/snmp"
	"github.com/td-anand/ribbon-snmp-forwarder/pkg/models"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to YAML configuration file")
	flag.Parse()

	// Bootstrap a temporary logger for startup messages.
	startupLog, _ := zap.NewProduction()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		startupLog.Fatal("failed to load configuration", zap.Error(err))
	}

	log, err := buildLogger(cfg.Log)
	if err != nil {
		startupLog.Fatal("failed to build logger", zap.Error(err))
	}
	defer log.Sync() //nolint:errcheck

	log.Info("ribbon-snmp-forwarder starting",
		zap.String("config", *cfgPath),
		zap.String("log_level", cfg.Log.Level),
	)

	// Kafka producer
	producer, err := kafka.NewProducer(cfg.Kafka, log)
	if err != nil {
		log.Fatal("failed to create Kafka producer", zap.Error(err))
	}
	defer producer.Close() //nolint:errcheck

	// TelemetryEvent handler: publishes every event to Kafka.
	handler := func(ctx context.Context, event models.TelemetryEvent) {
		if err := producer.Publish(ctx, event); err != nil {
			log.Error("failed to publish telemetry event",
				zap.String("source", string(event.Source)),
				zap.Error(err),
			)
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// SNMP trap receiver
	trapReceiver, err := snmp.NewTrapReceiver(cfg.SNMP.Trap, handler, log)
	if err != nil {
		log.Fatal("failed to create trap receiver", zap.Error(err))
	}

	// SNMP poller
	poller := snmp.NewPoller(cfg.SNMP.Poll, handler, log)

	// Run components concurrently; any fatal error cancels the whole service.
	errCh := make(chan error, 2)

	go func() {
		if err := trapReceiver.Start(ctx); err != nil {
			errCh <- fmt.Errorf("trap receiver: %w", err)
		}
	}()

	go func() {
		if err := poller.Start(ctx); err != nil {
			errCh <- fmt.Errorf("poller: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received – exiting")
	case err := <-errCh:
		log.Error("component error – shutting down", zap.Error(err))
		cancel()
	}
}

// buildLogger constructs a zap.Logger from the LogConfig.
func buildLogger(cfg config.LogConfig) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", cfg.Level, err)
	}

	var zapCfg zap.Config
	if cfg.Format == "console" {
		zapCfg = zap.NewDevelopmentConfig()
	} else {
		zapCfg = zap.NewProductionConfig()
	}
	zapCfg.Level = zap.NewAtomicLevelAt(level)

	return zapCfg.Build()
}
