#!/bin/bash
# Start EQUA Network - Full Stack

set -e

echo "🔷 EQUA Chain - Starting Network"
echo "================================"

cd "$(dirname "$0")"

# Check if initialized
if [ ! -f "genesis/equa-genesis.json" ]; then
    echo "❌ Genesis not found. Run init-validators.sh first."
    exit 1
fi

# Start network
echo ""
echo "🚀 Starting EQUA network (5 validators + 5 beacon engines)..."
docker-compose up -d

echo ""
echo "⏳ Waiting for validators to start..."
sleep 10

echo ""
echo "📊 Network Status:"
docker-compose ps

echo ""
echo "✅ Network started successfully!"
echo ""
echo "Available endpoints:"
echo "  Validator 1: http://localhost:8545 (RPC), http://localhost:8546 (WS)"
echo "  Validator 2: http://localhost:8547 (RPC)"
echo "  Validator 3: http://localhost:8548 (RPC)"
echo "  Validator 4: http://localhost:8549 (RPC)"
echo "  Validator 5: http://localhost:8550 (RPC)"
echo ""
echo "Useful commands:"
echo "  View logs: docker-compose logs -f [beacon1|validator1]"
echo "  Stop network: docker-compose down"
echo "  Restart: docker-compose restart"
echo "  Clean restart: docker-compose down -v && ./start-network.sh"
echo ""
echo "Check consensus:"
echo "  curl http://localhost:8545 -X POST -H 'Content-Type: application/json' \\"
echo "    --data '{\"jsonrpc\":\"2.0\",\"method\":\"equa_getConsensusStatus\",\"params\":[],\"id\":1}'"
