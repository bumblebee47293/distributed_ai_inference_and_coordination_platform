module github.com/yourusername/ai-platform/model-router

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/sony/gobreaker v0.5.0
	github.com/prometheus/client_golang v1.18.0
	go.opentelemetry.io/otel v1.21.0
	go.opentelemetry.io/otel/exporters/jaeger v1.17.0
	go.opentelemetry.io/otel/sdk v1.21.0
	go.opentelemetry.io/otel/trace v1.21.0
	go.uber.org/zap v1.26.0
	google.golang.org/grpc v1.60.1
)
