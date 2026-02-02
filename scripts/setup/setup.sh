#!/bin/bash

# Setup script for local development environment

set -e

echo "🚀 Setting up Distributed AI Inference Platform..."

# Check prerequisites
echo "📋 Checking prerequisites..."

command -v go >/dev/null 2>&1 || { echo "❌ Go is not installed. Please install Go 1.21+"; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "❌ Docker is not installed. Please install Docker"; exit 1; }
command -v docker-compose >/dev/null 2>&1 || { echo "❌ Docker Compose is not installed. Please install Docker Compose"; exit 1; }

echo "✅ Prerequisites check passed"

# Initialize Go modules
echo "📦 Initializing Go modules..."
cd services/api-gateway && go mod download && cd ../..
cd services/model-router && go mod download && cd ../..
cd services/inference-orchestrator && go mod download && cd ../..

echo "✅ Go modules initialized"

# Setup Python environment for models
echo "🐍 Setting up Python environment..."
if command -v python3 >/dev/null 2>&1; then
    cd models/sample-classifier
    python3 -m pip install -r requirements.txt
    echo "✅ Python dependencies installed"
    cd ../..
else
    echo "⚠️  Python3 not found. Skipping model setup."
fi

# Create necessary directories
echo "📁 Creating directories..."
mkdir -p bin
mkdir -p logs
mkdir -p data

echo "✅ Directories created"

# Build services
echo "🔨 Building services..."
make build

echo "✅ Services built successfully"

echo ""
echo "✨ Setup complete! ✨"
echo ""
echo "Next steps:"
echo "  1. Start infrastructure: docker-compose up -d"
echo "  2. Export sample model: cd models/sample-classifier && python export_model.py"
echo "  3. Test API: curl http://localhost:8080/health"
echo ""
echo "For more information, see README.md"
