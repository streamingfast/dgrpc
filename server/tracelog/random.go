package tracelog

import (
	"context"

	sftracing "github.com/streamingfast/sf-tracing"
	"go.opentelemetry.io/otel/propagation"
)

var noFields = []string{}

// RandomTrace is a fallback to insert a random trace ID if none was acquired from previous header lookups
type RandomTraceGetter struct{}

var _ propagation.TextMapPropagator = RandomTraceGetter{}

// Inject does not inject anything in RandomTraceGetter
func (p RandomTraceGetter) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {}

// Fields returns an empty list in RandomTraceGetter
func (p RandomTraceGetter) Fields() []string {
	return noFields
}

// Extract gives you a context with random TraceID and SpanID inserted if it didn't exist yet
func (p RandomTraceGetter) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	if sftracing.GetTraceID(ctx).IsValid() {
		return ctx
	}
	return sftracing.WithTraceID(ctx, sftracing.NewRandomTraceID())
}
