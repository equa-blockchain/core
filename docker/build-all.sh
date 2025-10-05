#!/bin/bash
# Build EQUA Network - Compilation Script

set -e

echo "🔷 EQUA Chain - Build All Components"
echo "===================================="

# Navigate to project root
cd "$(dirname "$0")/.."

# Build geth (execution layer)
echo ""
echo "📦 Building Geth (Execution Layer)..."
make geth

# Build beacon engine (consensus layer)
echo ""
echo "📦 Building EQUA Beacon Engine (Consensus Layer)..."
make beacon

# Build Docker images
echo ""
echo "🐳 Building Docker Images..."
cd docker

echo "  → Building Geth Docker image..."
docker build -t equa-chain:latest -f Dockerfile ..

echo "  → Building Beacon Engine Docker image..."
docker build -t equa-beacon:latest -f Dockerfile.beacon ..

echo ""
echo "✅ Build complete!"
echo ""
echo "Next steps:"
echo "  1. Initialize validators: cd docker && ./init-validators.sh"
echo "  2. Start network: docker-compose up -d"
echo "  3. Monitor logs: docker-compose logs -f beacon1"
