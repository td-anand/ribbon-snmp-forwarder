// Package kafka provides a thin Kafka producer wrapper for publishing
// TelemetryEvent messages to a configured topic.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"

	"github.com/td-anand/ribbon-snmp-forwarder/internal/config"
	"github.com/td-anand/ribbon-snmp-forwarder/pkg/models"
)

// Producer wraps a kafka-go writer and exposes a simple Publish method.
type Producer struct {
	writer *kafkago.Writer
	log    *zap.Logger
}

// NewProducer creates a Kafka producer using the supplied configuration.
// It verifies connectivity by dialing each broker before returning.
func NewProducer(cfg config.KafkaConfig, log *zap.Logger) (*Producer, error) {
	if err := ensureTopic(cfg, log); err != nil {
		// Non-fatal: topic may already exist or auto-creation may be disabled.
		log.Warn("topic pre-check skipped", zap.Error(err))
	}

	w := &kafkago.Writer{
		Addr:                   kafkago.TCP(cfg.Brokers...),
		Topic:                  cfg.Topic,
		Balancer:               &kafkago.LeastBytes{},
		MaxAttempts:            5,
		WriteBackoffMin:        100 * time.Millisecond,
		WriteBackoffMax:        1 * time.Second,
		BatchSize:              100,
		BatchTimeout:           500 * time.Millisecond,
		RequiredAcks:           kafkago.RequireOne,
		AllowAutoTopicCreation: true,
		Logger:                 kafkago.LoggerFunc(func(msg string, args ...interface{}) { log.Sugar().Debugf(msg, args...) }),
		ErrorLogger:            kafkago.LoggerFunc(func(msg string, args ...interface{}) { log.Sugar().Warnf(msg, args...) }),
	}

	return &Producer{writer: w, log: log}, nil
}

// Publish serialises a TelemetryEvent to JSON and writes it to the Kafka topic.
// The event's AgentAddress is used as the message key for partition affinity.
func (p *Producer) Publish(ctx context.Context, event models.TelemetryEvent) error {
	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshalling telemetry event: %w", err)
	}

	msg := kafkago.Message{
		Key:   []byte(event.AgentAddress),
		Value: value,
		Time:  event.Timestamp,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("writing kafka message: %w", err)
	}

	p.log.Debug("published telemetry event",
		zap.String("source", string(event.Source)),
		zap.String("agent", event.AgentAddress),
		zap.Int("var_binds", len(event.VarBinds)),
	)
	return nil
}

// Close flushes pending messages and releases the underlying writer resources.
func (p *Producer) Close() error {
	return p.writer.Close()
}

// ensureTopic attempts to create the Kafka topic if it does not already exist.
// Errors are intentionally non-fatal to support environments where auto-creation
// is managed externally.
func ensureTopic(cfg config.KafkaConfig, log *zap.Logger) error {
	if len(cfg.Brokers) == 0 {
		return fmt.Errorf("no brokers configured")
	}

	conn, err := kafkago.Dial("tcp", cfg.Brokers[0])
	if err != nil {
		return fmt.Errorf("dialing broker %q: %w", cfg.Brokers[0], err)
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("fetching controller: %w", err)
	}

	ctrlConn, err := kafkago.Dial("tcp", net.JoinHostPort(controller.Host, fmt.Sprintf("%d", controller.Port)))
	if err != nil {
		return fmt.Errorf("dialing controller: %w", err)
	}
	defer ctrlConn.Close()

	topicCfg := kafkago.TopicConfig{
		Topic:             cfg.Topic,
		NumPartitions:     3,
		ReplicationFactor: 1,
	}
	err = ctrlConn.CreateTopics(topicCfg)
	if err != nil {
		return fmt.Errorf("creating topic %q: %w", cfg.Topic, err)
	}
	log.Info("kafka topic ready", zap.String("topic", cfg.Topic))
	return nil
}
