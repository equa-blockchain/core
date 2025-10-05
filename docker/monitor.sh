#!/bin/bash
# Monitor EQUA Network

set -e

echo "📊 EQUA Chain - Network Monitor"
echo "==============================="
echo ""

RPC_URL=${1:-http://localhost:8545}

# Function to call RPC
call_rpc() {
    local method=$1
    local params=${2:-[]}
    curl -s -X POST $RPC_URL \
        -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"params\":$params,\"id\":1}" \
        | jq -r '.result'
}

echo "🔍 Consensus Status:"
echo "-------------------"
call_rpc "equa_getConsensusStatus" | jq '.'

echo ""
echo "📈 Network Stats:"
echo "----------------"
echo "Block Number: $(call_rpc "eth_blockNumber")"
echo "Peer Count: $(call_rpc "net_peerCount")"
echo "Mining: $(call_rpc "eth_mining")"

echo ""
echo "👥 Validators:"
echo "-------------"
call_rpc "equa_getValidators" | jq -r '.[] | "Address: \(.address) | Stake: \(.stake) | Active: \(.active)"'

echo ""
echo "🔥 MEV Stats (last 10 blocks):"
echo "-----------------------------"
call_rpc "equa_getMEVStats" "[10]" | jq '.'

echo ""
echo "⚖️  Ordering Score (last 5 blocks):"
echo "----------------------------------"
for i in {0..4}; do
    block_number=$(call_rpc "eth_blockNumber")
    block_num=$((16#${block_number:2} - i))
    score=$(call_rpc "equa_getOrderingScore" "[$block_num]")
    echo "Block $block_num: $score" | jq -r '"Score: \(.score) | Fair: \(.fairOrdering)"'
done

echo ""
echo "💎 PoW Difficulty:"
echo "-----------------"
call_rpc "equa_getPoWDifficulty" | jq '.'

echo ""
echo "✅ Monitor complete. Run with: ./monitor.sh [RPC_URL]"
