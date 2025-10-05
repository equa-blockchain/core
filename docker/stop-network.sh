#!/bin/bash
# Stop EQUA Network

set -e

echo "🛑 EQUA Chain - Stopping Network"
echo "================================"

cd "$(dirname "$0")"

# Ask for confirmation if cleaning volumes
if [ "$1" == "--clean" ]; then
    echo ""
    echo "⚠️  WARNING: This will delete all blockchain data!"
    read -p "Are you sure? (yes/no): " confirm

    if [ "$confirm" != "yes" ]; then
        echo "Cancelled."
        exit 0
    fi

    echo ""
    echo "🧹 Stopping network and cleaning volumes..."
    docker-compose down -v

    echo "✅ Network stopped and data cleaned."
else
    echo ""
    echo "🛑 Stopping network (preserving data)..."
    docker-compose down

    echo "✅ Network stopped. Data preserved."
    echo ""
    echo "To clean all data, run: ./stop-network.sh --clean"
fi
