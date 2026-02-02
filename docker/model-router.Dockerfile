# Multi-stage build for Model Router
FROM golang:1.21-alpine AS builder

WORKDIR /build

COPY services/model-router/go.mod services/model-router/go.sum ./
RUN go mod download

COPY services/model-router/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o model-router ./cmd/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /build/model-router .

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

CMD ["./model-router"]
