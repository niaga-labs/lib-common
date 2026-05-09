package telemetry

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Config holds OpenTelemetry configuration
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string // e.g., "jaeger:4318" for OTLP HTTP
	SampleRate     float64
	Enabled        bool
}

// DefaultConfig returns default telemetry configuration from environment
func DefaultConfig(serviceName string) Config {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "jaeger:4318"
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	enabled := os.Getenv("OTEL_ENABLED") != "false"

	return Config{
		ServiceName:    serviceName,
		ServiceVersion: os.Getenv("APP_VERSION"),
		Environment:    env,
		OTLPEndpoint:   endpoint,
		SampleRate:     0.1, // 10% sampling in production
		Enabled:        enabled,
	}
}

// Telemetry manages OpenTelemetry resources
type Telemetry struct {
	config         Config
	tracerProvider *sdktrace.TracerProvider
	logger         *zap.Logger
}

// New creates a new Telemetry instance
func New(cfg Config, logger *zap.Logger) (*Telemetry, error) {
	t := &Telemetry{
		config: cfg,
		logger: logger,
	}

	if !cfg.Enabled {
		logger.Info("OpenTelemetry disabled, using noop tracer")
		return t, nil
	}

	if err := t.initTracer(); err != nil {
		return nil, err
	}

	return t, nil
}

// initTracer initializes the OpenTelemetry tracer
func (t *Telemetry) initTracer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create OTLP exporter
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(t.config.OTLPEndpoint),
		otlptracehttp.WithInsecure(), // Use HTTP instead of HTTPS for internal communication
	)
	if err != nil {
		return err
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(t.config.ServiceName),
			semconv.ServiceVersion(t.config.ServiceVersion),
			semconv.DeploymentEnvironment(t.config.Environment),
			attribute.String("service.namespace", "niaga"),
		),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
	)
	if err != nil {
		return err
	}

	// Configure sampler based on environment
	var sampler sdktrace.Sampler
	if t.config.Environment == "production" {
		sampler = sdktrace.TraceIDRatioBased(t.config.SampleRate)
	} else {
		sampler = sdktrace.AlwaysSample() // Sample everything in development
	}

	// Create tracer provider
	t.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider
	otel.SetTracerProvider(t.tracerProvider)

	// Set global propagator for distributed tracing
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	t.logger.Info("OpenTelemetry tracer initialized",
		zap.String("service", t.config.ServiceName),
		zap.String("endpoint", t.config.OTLPEndpoint),
		zap.String("environment", t.config.Environment),
	)

	return nil
}

// Tracer returns a tracer for the service
func (t *Telemetry) Tracer() trace.Tracer {
	if t.tracerProvider == nil {
		return otel.Tracer(t.config.ServiceName)
	}
	return t.tracerProvider.Tracer(t.config.ServiceName)
}

// Shutdown gracefully shuts down the tracer provider
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t.tracerProvider == nil {
		return nil
	}

	t.logger.Info("Shutting down OpenTelemetry tracer")
	return t.tracerProvider.Shutdown(ctx)
}

// StartSpan is a helper to start a new span
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return otel.Tracer("").Start(ctx, name, opts...)
}

// SpanFromContext returns the current span from context
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddSpanAttributes adds attributes to the current span
func AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordError records an error on the current span
func RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
}
