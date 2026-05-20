/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package subscriber

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	otelconfig "github.com/thunder-id/thunderid/internal/system/observability/opentelemetry"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const otelSubscriberComponentName = "OTelSubscriber"

// OTelSubscriber converts observability events to OpenTelemetry spans.
// This is a STATELESS subscriber - each event creates an instant span with the event's timestamp.
// No span state is maintained between events, making it safe for multi-request flows.
type OTelSubscriber struct {
	id             string
	tracer         trace.Tracer
	tracerProvider *sdktrace.TracerProvider
	logger         *log.Logger
	categories     []event.EventCategory
}

var _ SubscriberInterface = (*OTelSubscriber)(nil)

// init registers the OTel subscriber factory with the global registry.
// This runs before main() and only registers the factory function.
// No configuration access or instance creation happens here.
func init() {
	RegisterSubscriberFactory("otel", func() SubscriberInterface {
		return NewOTelSubscriber()
	})
}

// NewOTelSubscriber creates a new OTel subscriber instance.
func NewOTelSubscriber() *OTelSubscriber {
	return &OTelSubscriber{}
}

// IsEnabled checks if the OTel subscriber should be activated based on configuration.
func (o *OTelSubscriber) IsEnabled() bool {
	return config.GetServerRuntime().Config.Observability.Output.OpenTelemetry.Enabled
}

// Initialize sets up the OTel subscriber with the provided configuration.
func (o *OTelSubscriber) Initialize() error {
	// Get config from observability service
	otelConfig := config.GetServerRuntime().Config.Observability.Output.OpenTelemetry

	o.logger = log.GetLogger().With(log.String(log.LoggerKeyComponentName, otelSubscriberComponentName))

	o.logger.Debug("Initializing OpenTelemetry subscriber",
		log.String("exporterType", otelConfig.ExporterType),
		log.String("endpoint", otelConfig.OTLPEndpoint))

	// Convert our config to otel package config
	otelProviderCfg := otelconfig.Config{
		Enabled:        otelConfig.Enabled,
		ExporterType:   otelConfig.ExporterType,
		OTLPEndpoint:   otelConfig.OTLPEndpoint,
		ServiceName:    otelConfig.ServiceName,
		ServiceVersion: otelConfig.ServiceVersion,
		Environment:    otelConfig.Environment,
		SampleRate:     otelConfig.SampleRate,
		Insecure:       otelConfig.Insecure,
	}

	// Create OTel tracer provider using the Initialize pattern
	ctx := context.Background()
	tracerProvider, err := otelconfig.Initialize(ctx, otelProviderCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize OpenTelemetry provider: %w", err)
	}

	// Store provider for shutdown
	o.tracerProvider = tracerProvider

	// Get tracer from the global provider
	o.tracer = otel.Tracer("thunderid-observability")
	o.id, err = utils.GenerateUUIDv7()
	if err != nil {
		o.logger.Error("Failed to generate UUID", log.Error(err))
		return fmt.Errorf("Failed to generate UUID: %w", err)
	}

	o.logger.Debug("OpenTelemetry subscriber initialized successfully",
		log.String("exporterType", otelConfig.ExporterType),
		log.String("serviceName", otelConfig.ServiceName))

	o.categories = convertCategories(otelConfig.Categories)
	if len(o.categories) == 0 {
		o.categories = []event.EventCategory{event.CategoryAll}
	}

	return nil
}

// GetID returns the unique identifier for this subscriber.
func (o *OTelSubscriber) GetID() string {
	return o.id
}

// GetCategories returns the configured categories for this subscriber.
func (o *OTelSubscriber) GetCategories() []event.EventCategory {
	return o.categories
}

// OnEvent processes an event and creates an OTel span.
// This implementation follows OpenTelemetry best practices:
// - Adds event metadata as span attributes (describes the span)
// - Adds event data as span events (represents what happened at a point in time)
//
// Note: Currently each event creates an independent span. For proper distributed tracing
// with span hierarchies, context propagation would need to be implemented separately.
func (o *OTelSubscriber) OnEvent(evt *event.Event) error {
	if evt == nil {
		return fmt.Errorf("event is nil")
	}

	return o.createSpan(evt)
}

// createSpan creates a span with span events for the event data.
// Following OTel best practices: event data becomes a span event (timestamp is meaningful).
func (o *OTelSubscriber) createSpan(evt *event.Event) error {
	ctx := context.Background()

	// Parse TraceID from event
	// OTel expects 32-char hex string, but Event TraceID might be UUID (36 chars with hyphens)
	cleanTraceID := strings.ReplaceAll(evt.TraceID, "-", "")
	traceID, err := trace.TraceIDFromHex(cleanTraceID)
	if err != nil {
		// If invalid TraceID, log warning and let OTel generate a new one
		o.logger.Warn("Invalid TraceID in event, generating new one",
			log.String("traceID", evt.TraceID),
			log.Error(err))
	} else {
		// Check for parent span ID in event data
		var parentSpanID trace.SpanID
		if parentIDStr := o.getStringData(evt, event.DataKey.TraceParent); parentIDStr != "" {
			if parsedParentID, err := trace.SpanIDFromHex(parentIDStr); err == nil {
				parentSpanID = parsedParentID
			}
		}

		// Create a remote span context with the event's TraceID
		spanContextConfig := trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     parentSpanID, // Can be empty if no parent
			TraceFlags: trace.FlagsSampled,
		}

		// If we have a parent span ID, we use it. If not, we still use the TraceID
		// but let OTel generate a new SpanID for this span.
		// However, to force OTel to use our TraceID, we must provide a remote span context.
		// If SpanID is zero, it might be treated as a root span of that trace.

		remoteSpanContext := trace.NewSpanContext(spanContextConfig)

		// Inject into context
		ctx = trace.ContextWithRemoteSpanContext(ctx, remoteSpanContext)
	}

	// Create span name from event type
	spanName := evt.Type

	// Build span attributes - these describe the SPAN itself (metadata)
	spanAttrs := []attribute.KeyValue{
		attribute.String("event.id", evt.EventID),
		attribute.String("trace.id", evt.TraceID),
		attribute.String("component", evt.Component),
		attribute.String("event.status", evt.Status),
	}

	// Determine span kind based on component
	spanKind := trace.SpanKindInternal
	if evt.Component == "OAuth2Server" || evt.Component == "TokenHandler" {
		spanKind = trace.SpanKindServer
	}

	// Start span
	_, span := o.tracer.Start(ctx, spanName,
		trace.WithTimestamp(evt.Timestamp),
		trace.WithSpanKind(spanKind),
		trace.WithAttributes(spanAttrs...),
	)

	// Convert event data to attributes
	// These will be used for both span tags (attributes) and span events (logs)
	eventAttrs := o.convertDataToAttributes(evt.Data)

	// Add event data as span attributes (tags) for searchability
	// This allows querying/filtering traces by event data fields
	span.SetAttributes(eventAttrs...)

	// Add span event with the event data (logs)
	// This represents "something that happened at a point in time" and preserves backward compatibility
	span.AddEvent(evt.Type,
		trace.WithTimestamp(evt.Timestamp),
		trace.WithAttributes(eventAttrs...),
	)

	// Set span status based on event status
	if evt.Status == event.StatusFailure {
		errorMsg := o.getStringData(evt, event.DataKey.Error)
		if errorMsg == "" {
			errorMsg = o.getStringData(evt, event.DataKey.FailureReason)
		}
		if errorMsg == "" {
			errorMsg = "unknown error"
		}
		span.SetStatus(codes.Error, errorMsg)
		span.RecordError(fmt.Errorf("%s", errorMsg),
			trace.WithTimestamp(evt.Timestamp))
	} else {
		span.SetStatus(codes.Ok, evt.Status)
	}

	// End span (duration = processing latency)
	// We use the current time (default behavior of End()) to capture how long
	// it took for the event to be processed by this subscriber.
	span.End()

	o.logger.Debug("Created span with event",
		log.String("spanName", spanName),
		log.String("eventType", evt.Type),
		log.String("status", evt.Status))

	return nil
}

// convertDataToAttributes converts event data map to OTel attributes.
func (o *OTelSubscriber) convertDataToAttributes(data map[string]interface{}) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(data))

	for key, value := range data {
		if value == nil {
			continue
		}

		// Convert value to appropriate attribute type
		switch v := value.(type) {
		case string:
			if v != "" {
				attrs = append(attrs, attribute.String(key, v))
			}
		case int:
			attrs = append(attrs, attribute.Int64(key, int64(v)))
		case int64:
			attrs = append(attrs, attribute.Int64(key, v))
		case float64:
			attrs = append(attrs, attribute.Float64(key, v))
		case bool:
			attrs = append(attrs, attribute.Bool(key, v))
		default:
			// Convert other types to string representation
			attrs = append(attrs, attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}

	return attrs
}

// getStringData safely extracts string data from event.
func (o *OTelSubscriber) getStringData(evt *event.Event, key string) string {
	if val, ok := evt.Data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// Close shuts down the tracer provider.
func (o *OTelSubscriber) Close() error {
	o.logger.Info("Closing OTel subscriber", log.String("subscriberID", o.id))

	if o.tracerProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := o.tracerProvider.Shutdown(ctx); err != nil {
			o.logger.Error("Failed to shutdown OpenTelemetry provider", log.Error(err))
			return err
		}
		o.logger.Debug("OpenTelemetry provider shutdown successfully")
		o.tracerProvider = nil
	}

	o.logger.Info("OTel subscriber closed successfully", log.String("subscriberID", o.id))
	return nil
}
