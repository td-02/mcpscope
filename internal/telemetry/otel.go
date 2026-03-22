package telemetry

import (
	"context"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"mcpscope/internal/intercept"
)

const (
	serviceName         = "mcpscope"
	defaultOTLPEndpoint = "localhost:4317"
)

type Client struct {
	tracer   trace.Tracer
	shutdown func(context.Context) error
}

func New(ctx context.Context, enabled bool) (*Client, error) {
	if !enabled {
		return noopClient(), nil
	}

	endpoint, ok := lookupEndpoint()
	if !ok {
		return noopClient(), nil
	}

	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(attribute.String("service.name", serviceName)),
	)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	return &Client{
		tracer: provider.Tracer(serviceName),
		shutdown: provider.Shutdown,
	}, nil
}

func (c *Client) Shutdown(ctx context.Context) error {
	if c == nil || c.shutdown == nil {
		return nil
	}
	return c.shutdown(ctx)
}

func (c *Client) RecordCall(ctx context.Context, serverName string, event intercept.Event) {
	if c == nil || event.Method == "" {
		return
	}

	receivedAt := eventReceivedAt(event)
	sentAt := eventSentAt(event)
	_, span := c.tracer.Start(ctx, event.Method, trace.WithTimestamp(receivedAt))
	span.SetAttributes(
		attribute.String("server_name", serverName),
		attribute.String("method", event.Method),
		attribute.Int64("latency_ms", event.LatencyMs),
		attribute.Bool("is_error", event.IsError),
		attribute.String("trace_id", event.TraceID),
	)
	if event.IsError {
		span.SetStatus(codes.Error, event.ErrorMessage)
	}
	span.End(trace.WithTimestamp(sentAt))
}

func noopClient() *Client {
	provider := noop.NewTracerProvider()
	return &Client{
		tracer: provider.Tracer(serviceName),
		shutdown: func(context.Context) error {
			return nil
		},
	}
}

func lookupEndpoint() (string, bool) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		_ = defaultOTLPEndpoint
		return "", false
	}
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	return endpoint, true
}

func eventReceivedAt(event intercept.Event) time.Time {
	return time.Unix(0, event.ReceivedAtUnixN)
}

func eventSentAt(event intercept.Event) time.Time {
	return time.Unix(0, event.SentAtUnixN)
}
