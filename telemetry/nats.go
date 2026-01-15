package telemetry

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	natsTracingHeader = "otel-trace-context"
)

// TracedNATSPublisher wraps NATS publishing with tracing
type TracedNATSPublisher struct {
	conn        *nats.Conn
	js          nats.JetStreamContext
	serviceName string
	tracer      trace.Tracer
}

// NewTracedNATSPublisher creates a traced NATS publisher
func NewTracedNATSPublisher(conn *nats.Conn, js nats.JetStreamContext, serviceName string) *TracedNATSPublisher {
	return &TracedNATSPublisher{
		conn:        conn,
		js:          js,
		serviceName: serviceName,
		tracer:      otel.Tracer(serviceName + "/nats"),
	}
}

// Publish publishes a message with tracing
func (p *TracedNATSPublisher) Publish(ctx context.Context, subject string, data []byte) error {
	ctx, span := p.tracer.Start(ctx, "NATS Publish "+subject,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			semconv.MessagingDestinationName(subject),
			semconv.MessagingOperationPublish,
			attribute.Int("messaging.message.payload_size_bytes", len(data)),
		),
	)
	defer span.End()

	// Create message with trace context in header
	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  make(nats.Header),
	}

	// Inject trace context into message headers
	carrier := NATSHeaderCarrier(msg.Header)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	var err error
	if p.js != nil {
		_, err = p.js.PublishMsg(msg)
	} else {
		err = p.conn.PublishMsg(msg)
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// PublishJSON publishes JSON data with tracing
func (p *TracedNATSPublisher) PublishJSON(ctx context.Context, subject string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.Publish(ctx, subject, data)
}

// NATSHeaderCarrier implements propagation.TextMapCarrier for NATS headers
type NATSHeaderCarrier nats.Header

func (c NATSHeaderCarrier) Get(key string) string {
	values := nats.Header(c).Values(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (c NATSHeaderCarrier) Set(key, value string) {
	nats.Header(c).Set(key, value)
}

func (c NATSHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// ExtractNATSContext extracts trace context from NATS message
func ExtractNATSContext(ctx context.Context, msg *nats.Msg) context.Context {
	if msg.Header == nil {
		return ctx
	}
	carrier := NATSHeaderCarrier(msg.Header)
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// StartConsumerSpan starts a span for NATS message consumption
func StartConsumerSpan(ctx context.Context, serviceName, subject string, msg *nats.Msg) (context.Context, trace.Span) {
	tracer := otel.Tracer(serviceName + "/nats")

	// Extract parent context from message
	ctx = ExtractNATSContext(ctx, msg)

	return tracer.Start(ctx, "NATS Consume "+subject,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			semconv.MessagingDestinationName(subject),
			semconv.MessagingOperationReceive,
			attribute.Int("messaging.message.payload_size_bytes", len(msg.Data)),
		),
	)
}

// TracedMessageHandler wraps a NATS message handler with tracing
type TracedMessageHandler struct {
	serviceName string
	handler     func(ctx context.Context, msg *nats.Msg) error
}

// NewTracedMessageHandler creates a traced message handler
func NewTracedMessageHandler(serviceName string, handler func(ctx context.Context, msg *nats.Msg) error) *TracedMessageHandler {
	return &TracedMessageHandler{
		serviceName: serviceName,
		handler:     handler,
	}
}

// Handle processes a message with tracing
func (h *TracedMessageHandler) Handle(msg *nats.Msg) {
	ctx, span := StartConsumerSpan(context.Background(), h.serviceName, msg.Subject, msg)
	defer span.End()

	err := h.handler(ctx, msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
}
