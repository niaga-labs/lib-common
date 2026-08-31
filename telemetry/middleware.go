package telemetry

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware returns a Gin middleware for HTTP request tracing
func TracingMiddleware(serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(c *gin.Context) {
		// Extract trace context from incoming request headers
		ctx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		// Create span name from route
		spanName := c.FullPath()
		if spanName == "" {
			spanName = c.Request.URL.Path
		}
		spanName = fmt.Sprintf("%s %s", c.Request.Method, spanName)

		// Start a new span
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPMethod(c.Request.Method),
				semconv.HTTPRoute(c.FullPath()),
				semconv.HTTPURL(c.Request.URL.String()),
				semconv.HTTPScheme(c.Request.URL.Scheme),
				semconv.NetHostName(c.Request.Host),
				semconv.UserAgentOriginal(c.Request.UserAgent()),
				semconv.HTTPRequestContentLength(int(c.Request.ContentLength)),
				attribute.String("http.client_ip", c.ClientIP()),
			),
		)
		defer span.End()

		// Add trace ID to response headers for debugging
		traceID := span.SpanContext().TraceID().String()
		c.Header("X-Trace-ID", traceID)

		// Store trace context in Gin context for downstream use
		c.Request = c.Request.WithContext(ctx)
		c.Set("traceID", traceID)
		c.Set("spanContext", span.SpanContext())

		// Process request
		c.Next()

		// Record response attributes
		statusCode := c.Writer.Status()
		span.SetAttributes(
			semconv.HTTPStatusCode(statusCode),
			semconv.HTTPResponseContentLength(c.Writer.Size()),
		)

		// Set span status based on HTTP status code
		if statusCode >= 400 && statusCode < 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("Client error: %d", statusCode))
		} else if statusCode >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("Server error: %d", statusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}

		// Record any errors that occurred during request processing
		if len(c.Errors) > 0 {
			for _, err := range c.Errors {
				span.RecordError(err.Err)
			}
		}
	}
}

// GetTraceID returns the trace ID from Gin context
func GetTraceID(c *gin.Context) string {
	if traceID, exists := c.Get("traceID"); exists {
		return traceID.(string)
	}
	return ""
}

// GetSpanContext returns the span context from Gin context
func GetSpanContext(c *gin.Context) trace.SpanContext {
	if sc, exists := c.Get("spanContext"); exists {
		return sc.(trace.SpanContext)
	}
	return trace.SpanContext{}
}

// InjectTracingHeaders injects trace context into outgoing request headers
func InjectTracingHeaders(ctx *gin.Context, req *http.Request) {
	otel.GetTextMapPropagator().Inject(ctx.Request.Context(), propagation.HeaderCarrier(req.Header))
}
