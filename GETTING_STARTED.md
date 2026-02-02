# Getting Started Guide

## Quick Start (5 minutes)

### 1. Verify Prerequisites

```bash
# Check Go installation
go version  # Should be 1.21+

# Check Docker
docker --version
docker-compose --version

# Check Python (for model export)
python3 --version
```

### 2. Clone and Setup

```bash
cd distributed_ai_inference_and_coordination_platform

# Make setup script executable
chmod +x scripts/setup/setup.sh

# Run setup (downloads dependencies, builds services)
./scripts/setup/setup.sh
```

### 3. Start the Platform

```bash
# Start all infrastructure and services
docker-compose up -d

# Check service health
docker-compose ps

# View logs
docker-compose logs -f api-gateway
```

### 4. Test the API

```bash
# Health check
curl http://localhost:8080/health

# Submit inference request
curl -X POST http://localhost:8080/v1/infer \
  -H "Authorization: Bearer demo-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "resnet18",
    "version": "v1",
    "input": {
      "image": "base64_image_data"
    }
  }'

# Submit batch job
curl -X POST http://localhost:8080/v1/batch \
  -H "Authorization: Bearer demo-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "resnet18",
    "version": "v1",
    "inputs": [
      {"image": "data1"},
      {"image": "data2"}
    ]
  }'
```

---

## Development Workflow

### Building Services

```bash
# Build all services
make build

# Build specific service
cd services/api-gateway && go build -o ../../bin/api-gateway ./cmd/main.go

# Run locally (without Docker)
./bin/api-gateway
```

### Running Tests

```bash
# Unit tests
make test

# With coverage
make test-coverage

# Integration tests
make test-integration

# Linting
make lint
```

### Working with Models

```bash
# Export sample model
cd models/sample-classifier
python3 export_model.py

# Organize for Triton
mkdir -p ../resnet18/1
mv resnet18.onnx ../resnet18/1/model.onnx
cp config.pbtxt ../resnet18/
```

---

## Observability

### Prometheus Metrics

Access at: http://localhost:9090

**Key queries:**

```promql
# Request rate
rate(http_requests_total[5m])

# P95 latency
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# Error rate
rate(http_requests_total{status=~"5.."}[5m])
```

### Jaeger Tracing

Access at: http://localhost:16686

- View end-to-end request traces
- Identify performance bottlenecks
- Debug distributed issues

### Logs

```bash
# View all logs
docker-compose logs -f

# Specific service
docker-compose logs -f api-gateway

# Follow with grep
docker-compose logs -f | grep ERROR
```

---

## Kubernetes Deployment

### Local Deployment (Minikube/Kind)

```bash
# Start minikube
minikube start

# Build and load images
make docker-build
minikube image load ai-platform/api-gateway:latest
minikube image load ai-platform/model-router:latest
minikube image load ai-platform/inference-orchestrator:latest

# Deploy
kubectl apply -k k8s/overlays/dev

# Check status
kubectl get pods -n ai-platform
kubectl get svc -n ai-platform

# Port forward to access
kubectl port-forward svc/api-gateway 8080:80 -n ai-platform
```

### Watch Autoscaling

```bash
# Generate load
k6 run scripts/loadtest/inference.js

# Watch HPA in another terminal
kubectl get hpa -n ai-platform -w

# Watch pods scaling
kubectl get pods -n ai-platform -w
```

---

## Load Testing

### Install k6

```bash
# macOS
brew install k6

# Linux
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update
sudo apt-get install k6
```

### Run Load Tests

```bash
# Basic test
k6 run scripts/loadtest/inference.js

# With custom target
k6 run --env BASE_URL=http://your-server:8080 scripts/loadtest/inference.js

# Smoke test (quick validation)
k6 run --vus 1 --duration 30s scripts/loadtest/inference.js

# Stress test
k6 run --vus 500 --duration 5m scripts/loadtest/inference.js
```

---

## Troubleshooting

### Services Won't Start

```bash
# Check Docker resources
docker system df

# Clean up
docker-compose down -v
docker system prune -a

# Restart
docker-compose up -d
```

### Port Conflicts

```bash
# Find process using port
lsof -i :8080  # macOS/Linux
netstat -ano | findstr :8080  # Windows

# Kill process or change port in docker-compose.yml
```

### Database Connection Issues

```bash
# Check PostgreSQL
docker-compose exec postgres psql -U admin -d aiplatform

# Check Redis
docker-compose exec redis redis-cli ping
```

### Kafka Issues

```bash
# Check Kafka topics
docker-compose exec kafka kafka-topics --list --bootstrap-server localhost:9092

# Create topic manually
docker-compose exec kafka kafka-topics --create \
  --topic inference-jobs \
  --bootstrap-server localhost:9092 \
  --partitions 3 \
  --replication-factor 1
```

---

## Next Steps

### For Development

1. **Add more models**: Export additional ONNX models
2. **Implement Metadata Service**: Complete model registry with PostgreSQL
3. **Add Batch Worker**: Implement Kafka consumer for async jobs
4. **Enhance routing**: Add latency-based and canary routing
5. **Add authentication**: Implement proper JWT token generation

### For Production

1. **Set up monitoring**: Configure Grafana dashboards
2. **Add alerting**: Set up Prometheus alerts
3. **Implement secrets**: Use Kubernetes secrets or Vault
4. **Configure TLS**: Add HTTPS support
5. **Set up CI/CD**: Configure GitHub Actions with your registry

### For Interviews

1. **Document architecture decisions**: Add ADRs (Architecture Decision Records)
2. **Create diagrams**: Add sequence diagrams for key flows
3. **Write blog post**: Explain your design choices
4. **Record demo**: Show the platform in action
5. **Prepare talking points**: Be ready to discuss tradeoffs

---

## Useful Commands

```bash
# Stop all services
docker-compose down

# Stop and remove volumes
docker-compose down -v

# Rebuild specific service
docker-compose up -d --build api-gateway

# Scale a service
docker-compose up -d --scale batch-worker=3

# View resource usage
docker stats

# Clean everything
make clean
docker-compose down -v
docker system prune -a
```

---

## Resources

- **Go Documentation**: https://golang.org/doc/
- **Triton Inference Server**: https://github.com/triton-inference-server/server
- **Kubernetes**: https://kubernetes.io/docs/
- **Prometheus**: https://prometheus.io/docs/
- **OpenTelemetry**: https://opentelemetry.io/docs/

---

## Support

- Open an issue for bugs
- Start a discussion for questions
- Check existing issues before creating new ones

**Happy coding! 🚀**
