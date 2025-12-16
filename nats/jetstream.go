package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// StreamConfig defines stream configuration
type StreamConfig struct {
	Name        string
	Description string
	Subjects    []string
	MaxAge      time.Duration // How long to keep messages
	MaxMsgs     int64         // Max messages per subject
	MaxBytes    int64         // Max total bytes
	Replicas    int           // Number of replicas (1 for single node)
}

// ConsumerConfig defines consumer configuration
type ConsumerConfig struct {
	Name          string
	Stream        string
	FilterSubject string
	AckWait       time.Duration
	MaxDeliver    int
	MaxAckPending int
}

// JetStreamClient wraps NATS JetStream functionality
type JetStreamClient struct {
	nc     *nats.Conn
	js     jetstream.JetStream
	logger *zap.Logger
}

// Default stream configurations for Kilang services
var DefaultStreams = []StreamConfig{
	{
		Name:        "ORDERS",
		Description: "Order lifecycle events",
		Subjects:    []string{"order.>"},
		MaxAge:      7 * 24 * time.Hour, // Keep for 7 days
		MaxMsgs:     100000,
		MaxBytes:    100 * 1024 * 1024, // 100MB
		Replicas:    1,
	},
	{
		Name:        "INVENTORY",
		Description: "Inventory and stock events",
		Subjects:    []string{"inventory.>"},
		MaxAge:      7 * 24 * time.Hour,
		MaxMsgs:     100000,
		MaxBytes:    100 * 1024 * 1024,
		Replicas:    1,
	},
	{
		Name:        "PRODUCTS",
		Description: "Product catalog events",
		Subjects:    []string{"product.>"},
		MaxAge:      7 * 24 * time.Hour,
		MaxMsgs:     50000,
		MaxBytes:    50 * 1024 * 1024,
		Replicas:    1,
	},
	{
		Name:        "NOTIFICATIONS",
		Description: "Notification events",
		Subjects:    []string{"events.>"},
		MaxAge:      3 * 24 * time.Hour, // Keep for 3 days
		MaxMsgs:     50000,
		MaxBytes:    50 * 1024 * 1024,
		Replicas:    1,
	},
}

// NewJetStreamClient creates a new JetStream client with retry logic
func NewJetStreamClient(natsURL string, logger *zap.Logger) (*JetStreamClient, error) {
	opts := []nats.Option{
		nats.Name("kilang-service"),
		nats.ReconnectWait(2 * time.Second),
		nats.MaxReconnects(-1), // Unlimited reconnects
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				logger.Warn("NATS disconnected", zap.Error(err))
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			logger.Error("NATS error", zap.Error(err))
		}),
	}

	// Connect with retry
	var nc *nats.Conn
	var err error
	for i := 0; i < 10; i++ {
		nc, err = nats.Connect(natsURL, opts...)
		if err == nil {
			break
		}
		logger.Warn("Failed to connect to NATS, retrying...",
			zap.Int("attempt", i+1),
			zap.Error(err))
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS after retries: %w", err)
	}

	// Create JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	logger.Info("Connected to NATS with JetStream", zap.String("url", natsURL))

	return &JetStreamClient{
		nc:     nc,
		js:     js,
		logger: logger,
	}, nil
}

// EnsureStream creates or updates a stream
func (c *JetStreamClient) EnsureStream(ctx context.Context, cfg StreamConfig) (jetstream.Stream, error) {
	streamCfg := jetstream.StreamConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Subjects:    cfg.Subjects,
		MaxAge:      cfg.MaxAge,
		MaxMsgs:     cfg.MaxMsgs,
		MaxBytes:    cfg.MaxBytes,
		Replicas:    cfg.Replicas,
		Storage:     jetstream.FileStorage, // Persist to disk
		Retention:   jetstream.LimitsPolicy,
		Discard:     jetstream.DiscardOld,
	}

	stream, err := c.js.CreateOrUpdateStream(ctx, streamCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create/update stream %s: %w", cfg.Name, err)
	}

	c.logger.Info("Stream ready",
		zap.String("name", cfg.Name),
		zap.Strings("subjects", cfg.Subjects))

	return stream, nil
}

// EnsureDefaultStreams creates all default streams
func (c *JetStreamClient) EnsureDefaultStreams(ctx context.Context) error {
	for _, cfg := range DefaultStreams {
		if _, err := c.EnsureStream(ctx, cfg); err != nil {
			return err
		}
	}
	return nil
}

// CreateDurableConsumer creates a durable consumer for reliable message delivery
func (c *JetStreamClient) CreateDurableConsumer(ctx context.Context, cfg ConsumerConfig) (jetstream.Consumer, error) {
	stream, err := c.js.Stream(ctx, cfg.Stream)
	if err != nil {
		return nil, fmt.Errorf("stream %s not found: %w", cfg.Stream, err)
	}

	consumerCfg := jetstream.ConsumerConfig{
		Name:          cfg.Name,
		Durable:       cfg.Name,
		FilterSubject: cfg.FilterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       cfg.AckWait,
		MaxDeliver:    cfg.MaxDeliver,
		MaxAckPending: cfg.MaxAckPending,
		DeliverPolicy: jetstream.DeliverAllPolicy, // Start from first message
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, consumerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer %s: %w", cfg.Name, err)
	}

	c.logger.Info("Consumer ready",
		zap.String("name", cfg.Name),
		zap.String("stream", cfg.Stream),
		zap.String("filter", cfg.FilterSubject))

	return consumer, nil
}

// Publish publishes a message to JetStream with acknowledgment
func (c *JetStreamClient) Publish(ctx context.Context, subject string, data []byte) error {
	ack, err := c.js.Publish(ctx, subject, data)
	if err != nil {
		c.logger.Error("Failed to publish message",
			zap.String("subject", subject),
			zap.Error(err))
		return err
	}

	c.logger.Debug("Message published",
		zap.String("subject", subject),
		zap.String("stream", ack.Stream),
		zap.Uint64("sequence", ack.Sequence))

	return nil
}

// PublishAsync publishes a message asynchronously
func (c *JetStreamClient) PublishAsync(subject string, data []byte) error {
	_, err := c.js.PublishAsync(subject, data)
	return err
}

// Subscribe creates a simple push-based subscription (for backward compatibility)
func (c *JetStreamClient) Subscribe(subject string, handler func(msg *nats.Msg)) (*nats.Subscription, error) {
	return c.nc.Subscribe(subject, handler)
}

// ConsumeMessages starts consuming messages from a consumer with a handler
func (c *JetStreamClient) ConsumeMessages(ctx context.Context, consumer jetstream.Consumer, handler func(msg jetstream.Msg)) (jetstream.ConsumeContext, error) {
	return consumer.Consume(func(msg jetstream.Msg) {
		handler(msg)
	})
}

// GetJetStream returns the underlying JetStream context
func (c *JetStreamClient) GetJetStream() jetstream.JetStream {
	return c.js
}

// GetConnection returns the underlying NATS connection
func (c *JetStreamClient) GetConnection() *nats.Conn {
	return c.nc
}

// Close closes the NATS connection
func (c *JetStreamClient) Close() {
	if c.nc != nil {
		c.nc.Drain()
		c.nc.Close()
		c.logger.Info("NATS connection closed")
	}
}

// HealthCheck checks if NATS and JetStream are healthy
func (c *JetStreamClient) HealthCheck(ctx context.Context) error {
	if !c.nc.IsConnected() {
		return fmt.Errorf("NATS not connected")
	}

	// Try to get account info to verify JetStream is working
	_, err := c.js.AccountInfo(ctx)
	if err != nil {
		return fmt.Errorf("JetStream not available: %w", err)
	}

	return nil
}
