# Multi-stage build for Batch Worker
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY services/batch-worker/go.mod services/batch-worker/go.sum* ./
RUN go mod download

# Copy source code
COPY services/batch-worker/ ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o batch-worker ./cmd/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/batch-worker .

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /root

USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD pgrep batch-worker || exit 1

ENTRYPOINT ["./batch-worker"]
