package middleware

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Tracing middleware adds OpenTelemetry tracing to requests
func Tracing() gin.HandlerFunc {
	tracer := otel.Tracer("api-gateway")

	return func(c *gin.Context) {
		ctx, span := tracer.Start(
			c.Request.Context(),
			c.Request.Method+" "+c.FullPath(),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// Set span attributes
		span.SetAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.url", c.Request.URL.String()),
			attribute.String("http.user_agent", c.Request.UserAgent()),
		)

		// Store trace ID in context for logging
		if span.SpanContext().HasTraceID() {
			c.Set("trace_id", span.SpanContext().TraceID().String())
		}

		// Update request context
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		// Set status code
		span.SetAttributes(attribute.Int("http.status_code", c.Writer.Status()))
	}
}
