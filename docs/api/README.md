# API Documentation

Complete OpenAPI/Swagger documentation for all AI Inference Platform services.

## 📚 Available APIs

### 1. **API Gateway** (`api-gateway.yaml`)

**Port**: 8080  
**Base URL**: `http://localhost:8080`

Main entry point for all client requests.

**Endpoints**:

- `POST /v1/infer` - Real-time inference
- `POST /v1/batch` - Submit batch job
- `GET /v1/batch/{jobId}` - Get job status
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics

**Features**:

- JWT authentication
- Rate limiting (100 req/min per IP)
- CORS support
- Distributed tracing

---

### 2. **Metadata Service** (`metadata-service.yaml`)

**Port**: 8083  
**Base URL**: `http://localhost:8083`

Model registry and metadata management.

**Endpoints**:

- `GET /v1/models` - List all models
- `POST /v1/models` - Register new model
- `GET /v1/models/{id}` - Get model by ID
- `PUT /v1/models/{id}` - Update model
- `DELETE /v1/models/{id}` - Delete model
- `GET /v1/models/by-name/{name}/{version}` - Get by name/version
- `GET /v1/models/{id}/stats` - Get model statistics

**Features**:

- Redis caching
- PostgreSQL persistence
- Version control
- Usage statistics

---

### 3. **Model Router** (`model-router.yaml`)

**Port**: 8081  
**Base URL**: `http://localhost:8081`

Intelligent routing with circuit breakers.

**Endpoints**:

- `POST /route` - Route inference request
- `GET /health` - Health check with backend status
- `GET /metrics` - Circuit breaker metrics

**Features**:

- Circuit breaker pattern
- Load balancing (round-robin)
- Backend health tracking
- Automatic failover

---

### 4. **Inference Orchestrator** (`inference-orchestrator.yaml`)

**Port**: 8082  
**Base URL**: `http://localhost:8082`

Triton Inference Server integration.

**Endpoints**:

- `POST /v1/infer` - Execute inference
- `GET /v1/models/{name}/ready` - Check model readiness
- `GET /health` - Health check

**Features**:

- Triton gRPC/HTTP integration
- Model health checking
- Request orchestration

---

## 🚀 Viewing the Documentation

### Option 1: Swagger UI (Recommended)

Run Swagger UI with Docker:

```bash
# API Gateway
docker run -p 8090:8080 -e SWAGGER_JSON=/docs/api-gateway.yaml \
  -v $(pwd)/docs/api:/docs swaggerapi/swagger-ui

# Metadata Service
docker run -p 8091:8080 -e SWAGGER_JSON=/docs/metadata-service.yaml \
  -v $(pwd)/docs/api:/docs swaggerapi/swagger-ui
```

Then open:

- API Gateway: http://localhost:8090
- Metadata Service: http://localhost:8091

### Option 2: Swagger Editor

```bash
docker run -p 8080:8080 swaggerapi/swagger-editor
```

Open http://localhost:8080 and paste the YAML content.

### Option 3: VS Code Extension

Install the **OpenAPI (Swagger) Editor** extension and open any `.yaml` file.

### Option 4: Online Viewer

Visit https://editor.swagger.io and paste the YAML content.

---

## 📖 Quick Start Examples

### Real-time Inference

```bash
curl -X POST http://localhost:8080/v1/infer \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer demo-token" \
  -d '{
    "model": "resnet18",
    "version": "1",
    "input": {
      "data": [1.0, 2.0, 3.0, 4.0]
    }
  }'
```

**Response**:

```json
{
  "prediction": [0.1, 0.9, 0.0, 0.0],
  "model": "resnet18",
  "version": "1",
  "latency_ms": 145,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

---

### Submit Batch Job

```bash
curl -X POST http://localhost:8080/v1/batch \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer demo-token" \
  -d '{
    "model": "resnet18",
    "version": "1",
    "inputs": [
      {"data": [1.0, 2.0, 3.0]},
      {"data": [4.0, 5.0, 6.0]},
      {"data": [7.0, 8.0, 9.0]}
    ]
  }'
```

**Response**:

```json
{
  "job_id": "job-123e4567-e89b-12d3-a456-426614174000",
  "status": "pending",
  "message": "Job submitted successfully",
  "created_at": "2024-01-15T10:30:00Z"
}
```

---

### Check Job Status

```bash
curl http://localhost:8080/v1/batch/job-123e4567-e89b-12d3-a456-426614174000 \
  -H "Authorization: Bearer demo-token"
```

**Response**:

```json
{
  "job_id": "job-123e4567-e89b-12d3-a456-426614174000",
  "status": "completed",
  "progress": 1.0,
  "total": 3,
  "completed": 3,
  "result_url": "https://minio.aiplatform.com/results/job-123.json",
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:31:00Z"
}
```

---

### Register a Model

```bash
curl -X POST http://localhost:8083/v1/models \
  -H "Content-Type: application/json" \
  -d '{
    "name": "resnet18",
    "version": "1.0.0",
    "framework": "pytorch",
    "format": "onnx",
    "backend_url": "http://triton:8001",
    "description": "ResNet-18 image classification",
    "tags": ["vision", "classification"],
    "metadata": {
      "input_shape": [1, 3, 224, 224],
      "output_shape": [1, 1000]
    }
  }'
```

---

## 🔐 Authentication

All API Gateway endpoints require authentication:

### Bearer Token (Development)

```bash
Authorization: Bearer demo-token
```

### API Key (Production)

```bash
X-API-Key: your-api-key-here
```

### JWT Token (Production)

```bash
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

---

## 📊 Response Codes

| Code | Meaning             | Description              |
| ---- | ------------------- | ------------------------ |
| 200  | OK                  | Request successful       |
| 201  | Created             | Resource created         |
| 202  | Accepted            | Request accepted (async) |
| 204  | No Content          | Successful deletion      |
| 400  | Bad Request         | Invalid input            |
| 401  | Unauthorized        | Missing/invalid auth     |
| 404  | Not Found           | Resource not found       |
| 409  | Conflict            | Resource already exists  |
| 429  | Too Many Requests   | Rate limit exceeded      |
| 500  | Internal Error      | Server error             |
| 503  | Service Unavailable | Circuit breaker open     |

---

## 🎯 Rate Limiting

API Gateway implements rate limiting:

- **Limit**: 100 requests per minute per IP
- **Headers**:
  - `X-RateLimit-Limit`: Total limit
  - `X-RateLimit-Remaining`: Remaining requests
  - `X-RateLimit-Reset`: Reset timestamp

**Example**:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1705320660
```

---

## 🔍 Distributed Tracing

All requests include tracing headers:

**Request Headers**:

```
X-Request-ID: custom-request-id
```

**Response Headers**:

```
X-Trace-ID: 1234567890abcdef
X-Request-ID: custom-request-id
```

View traces in Jaeger: http://localhost:16686

---

## 📈 Monitoring

### Prometheus Metrics

All services expose metrics at `/metrics`:

```bash
# API Gateway metrics
curl http://localhost:8080/metrics

# Metadata Service metrics
curl http://localhost:8083/metrics

# Model Router metrics
curl http://localhost:8081/metrics
```

View in Prometheus: http://localhost:9090

---

## 🧪 Testing with Postman

### Import Collection

1. Install Postman
2. Import OpenAPI spec:
   - File → Import → Upload `api-gateway.yaml`
3. Set environment variables:
   - `base_url`: `http://localhost:8080`
   - `auth_token`: `demo-token`

### Example Collection

```json
{
  "info": {
    "name": "AI Inference Platform",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "item": [
    {
      "name": "Real-time Inference",
      "request": {
        "method": "POST",
        "header": [
          { "key": "Authorization", "value": "Bearer {{auth_token}}" }
        ],
        "url": "{{base_url}}/v1/infer",
        "body": {
          "mode": "raw",
          "raw": "{\"model\":\"resnet18\",\"version\":\"1\",\"input\":{\"data\":[1.0,2.0,3.0]}}"
        }
      }
    }
  ]
}
```

---

## 📝 API Versioning

All APIs use URL-based versioning:

- Current version: `v1`
- Base path: `/v1/*`
- Future versions: `/v2/*`, `/v3/*`, etc.

**Backward compatibility** is maintained within major versions.

---

## 🛠️ Development

### Validate OpenAPI Specs

```bash
# Install validator
npm install -g @apidevtools/swagger-cli

# Validate specs
swagger-cli validate docs/api/api-gateway.yaml
swagger-cli validate docs/api/metadata-service.yaml
swagger-cli validate docs/api/model-router.yaml
swagger-cli validate docs/api/inference-orchestrator.yaml
```

### Generate Client SDKs

```bash
# Install OpenAPI Generator
npm install -g @openapitools/openapi-generator-cli

# Generate Python client
openapi-generator-cli generate \
  -i docs/api/api-gateway.yaml \
  -g python \
  -o clients/python

# Generate JavaScript client
openapi-generator-cli generate \
  -i docs/api/api-gateway.yaml \
  -g javascript \
  -o clients/javascript

# Generate Go client
openapi-generator-cli generate \
  -i docs/api/api-gateway.yaml \
  -g go \
  -o clients/go
```

---

## 📚 Additional Resources

- **OpenAPI Specification**: https://swagger.io/specification/
- **Swagger UI**: https://swagger.io/tools/swagger-ui/
- **Swagger Editor**: https://editor.swagger.io/
- **OpenAPI Generator**: https://openapi-generator.tech/

---

## ✅ API Documentation Checklist

- ✅ API Gateway - Complete with examples
- ✅ Metadata Service - Full CRUD operations
- ✅ Model Router - Circuit breaker details
- ✅ Inference Orchestrator - Triton integration
- ✅ Authentication documentation
- ✅ Rate limiting details
- ✅ Error responses
- ✅ Example requests/responses
- ✅ Distributed tracing
- ✅ Monitoring integration

---

**Status**: ✅ **100% API Documentation Complete!**

All services have comprehensive OpenAPI 3.0 specifications with:

- Detailed endpoint descriptions
- Request/response schemas
- Example payloads
- Error handling
- Authentication details
- Rate limiting information
