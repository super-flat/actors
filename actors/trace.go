package actors

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func getSpanContext(ctx context.Context, methodName string) (context.Context, trace.Span) {
	// Create a span
	tracer := otel.GetTracerProvider()
	spanCtx, span := tracer.Tracer("").Start(ctx, methodName)
	return spanCtx, span
}
