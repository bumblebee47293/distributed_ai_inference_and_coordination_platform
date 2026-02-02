# Multi-stage build for Inference Orchestrator
FROM golang:1.21-alpine AS builder

WORKDIR /build

COPY services/inference-orchestrator/go.mod services/inference-orchestrator/go.sum ./
RUN go mod download

COPY services/inference-orchestrator/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o inference-orchestrator ./cmd/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/inference-orchestrator .

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

EXPOSE 8082

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

CMD ["./inference-orchestrator"]
