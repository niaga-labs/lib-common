package telemetry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// TracedHTTPClient wraps http.Client with OpenTelemetry tracing
type TracedHTTPClient struct {
	client      *http.Client
	serviceName string
	tracer      trace.Tracer
}

// NewTracedHTTPClient creates a new HTTP client with tracing enabled
func NewTracedHTTPClient(serviceName string, timeout time.Duration) *TracedHTTPClient {
	return &TracedHTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		serviceName: serviceName,
		tracer:      otel.Tracer(serviceName),
	}
}

// NewTracedHTTPClientWithTransport creates a client with custom transport
func NewTracedHTTPClientWithTransport(serviceName string, timeout time.Duration, transport http.RoundTripper) *TracedHTTPClient {
	return &TracedHTTPClient{
		client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		serviceName: serviceName,
		tracer:      otel.Tracer(serviceName),
	}
}

// Do executes the request with tracing
func (c *TracedHTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	spanName := fmt.Sprintf("HTTP %s %s", req.Method, req.URL.Path)

	ctx, span := c.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.HTTPMethod(req.Method),
			semconv.HTTPURL(req.URL.String()),
			semconv.NetPeerName(req.URL.Host),
			attribute.String("http.target", req.URL.Path),
		),
	)
	defer span.End()

	// Inject trace context into outgoing request headers
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	// Execute request
	req = req.WithContext(ctx)
	resp, err := c.client.Do(req)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Record response attributes
	span.SetAttributes(
		semconv.HTTPStatusCode(resp.StatusCode),
		attribute.Int64("http.response_content_length", resp.ContentLength),
	)

	// Set span status based on response
	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return resp, nil
}

// Get performs a GET request with tracing
func (c *TracedHTTPClient) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req)
}

// Post performs a POST request with tracing
func (c *TracedHTTPClient) Post(ctx context.Context, url, contentType string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(ctx, req)
}

// PostJSON performs a POST request with JSON content type
func (c *TracedHTTPClient) PostJSON(ctx context.Context, url string, body []byte) (*http.Response, error) {
	return c.Post(ctx, url, "application/json", body)
}

// Put performs a PUT request with tracing
func (c *TracedHTTPClient) Put(ctx context.Context, url, contentType string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.Do(ctx, req)
}

// Delete performs a DELETE request with tracing
func (c *TracedHTTPClient) Delete(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(ctx, req)
}

// ReadBody reads and returns the response body, recording size in span
func ReadBody(ctx context.Context, resp *http.Response) ([]byte, error) {
	span := trace.SpanFromContext(ctx)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.response_body_size", len(body)))
	return body, nil
}
