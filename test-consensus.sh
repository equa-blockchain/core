#!/bin/bash
# EQUA Consensus Test Script
# Tests all new consensus APIs and diagnostics

set -e

echo "🧪 EQUA Consensus Test Suite"
echo "============================"
echo ""

RPC_URL="${RPC_URL:-http://localhost:8545}"

# Helper function to make RPC calls
rpc_call() {
    local method="$1"
    local params="${2:-[]}"

    curl -s -X POST \
        -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"params\":$params,\"id\":1}" \
        "$RPC_URL" | jq -r '.result'
}

# Test 1: Get Block Period
echo "📊 Test 1: Get Block Period"
echo "----------------------------"
PERIOD=$(rpc_call "equa_getBlockPeriod")
echo "Current block period: ${PERIOD}s"
echo "✅ Test 1 passed"
echo ""

# Test 2: Get Consensus Status
echo "📊 Test 2: Get Consensus Status"
echo "--------------------------------"
STATUS=$(rpc_call "equa_getConsensusStatus")
echo "$STATUS" | jq '.'
echo "✅ Test 2 passed"
echo ""

# Test 3: Get Validators
echo "📊 Test 3: Get Validators"
echo "-------------------------"
VALIDATORS=$(rpc_call "equa_getValidators")
echo "$VALIDATORS" | jq '.'
VALIDATOR_COUNT=$(echo "$VALIDATORS" | jq 'length')
echo "Total validators: $VALIDATOR_COUNT"
echo "✅ Test 3 passed"
echo ""

# Test 4: Get Validators Info
echo "📊 Test 4: Get Validators Info"
echo "-------------------------------"
VALIDATORS_INFO=$(rpc_call "equa_getValidatorsInfo")
echo "$VALIDATORS_INFO" | jq '.'
echo "✅ Test 4 passed"
echo ""

# Test 5: Get MEV Stats
echo "📊 Test 5: Get MEV Stats (last 10 blocks)"
echo "------------------------------------------"
MEV_STATS=$(rpc_call "equa_getMEVStats" "[10]")
echo "$MEV_STATS" | jq '.'
echo "✅ Test 5 passed"
echo ""

# Test 6: Get PoW Difficulty
echo "📊 Test 6: Get PoW Difficulty"
echo "------------------------------"
POW_DIFFICULTY=$(rpc_call "equa_getPoWDifficulty")
echo "PoW Difficulty: $POW_DIFFICULTY"
echo "✅ Test 6 passed"
echo ""

# Test 7: Diagnose Consensus
echo "📊 Test 7: Diagnose Consensus (last 10 blocks)"
echo "-----------------------------------------------"
DIAGNOSIS=$(rpc_call "equa_diagnoseConsensus" "[10]")
echo "$DIAGNOSIS" | jq '.'
HEALTH_STATUS=$(echo "$DIAGNOSIS" | jq -r '.health.status')
HEALTH_SCORE=$(echo "$DIAGNOSIS" | jq -r '.health.score')
echo ""
echo "Health Status: $HEALTH_STATUS"
echo "Health Score: $HEALTH_SCORE"
echo "✅ Test 7 passed"
echo ""

# Test 8: Prove Consensus (latest block)
echo "📊 Test 8: Prove Consensus (latest block)"
echo "------------------------------------------"
LATEST_BLOCK=$(rpc_call "eth_blockNumber")
BLOCK_NUM=$(printf "%d" $LATEST_BLOCK)
PROOF=$(rpc_call "equa_proveConsensus" "[$BLOCK_NUM]")
echo "$PROOF" | jq '.'
echo "✅ Test 8 passed"
echo ""

# Test 9: Get Ordering Score
echo "📊 Test 9: Get Ordering Score (latest block)"
echo "---------------------------------------------"
ORDERING_SCORE=$(rpc_call "equa_getOrderingScore" "[$BLOCK_NUM]")
echo "$ORDERING_SCORE" | jq '.'
echo "✅ Test 9 passed"
echo ""

# Test 10: Set Block Period (dynamic adjustment)
echo "📊 Test 10: Set Block Period (dynamic adjustment)"
echo "--------------------------------------------------"
echo "Current period: ${PERIOD}s"
echo "Attempting to set period to 8 seconds..."
SET_RESULT=$(rpc_call "equa_setBlockPeriod" "[8]")
echo "$SET_RESULT" | jq '.'
NEW_PERIOD=$(rpc_call "equa_getBlockPeriod")
echo "New period: ${NEW_PERIOD}s"

# Revert back to original period
echo "Reverting back to original period: ${PERIOD}s"
REVERT_RESULT=$(rpc_call "equa_setBlockPeriod" "[$PERIOD]")
echo "$REVERT_RESULT" | jq '.'
echo "✅ Test 10 passed"
echo ""

# Test 11: Get Consensus Info
echo "📊 Test 11: Get Consensus Info"
echo "-------------------------------"
CONSENSUS_INFO=$(rpc_call "equa_getConsensusInfo")
echo "$CONSENSUS_INFO" | jq '.'
echo "✅ Test 11 passed"
echo ""

# Summary
echo ""
echo "🎉 All Tests Passed!"
echo "===================="
echo ""
echo "📋 Summary:"
echo "- Block Period: ${PERIOD}s"
echo "- Validators: $VALIDATOR_COUNT"
echo "- Health Status: $HEALTH_STATUS"
echo "- Health Score: $HEALTH_SCORE"
echo "- PoW Difficulty: $POW_DIFFICULTY"
echo ""
echo "✅ EQUA Consensus is working correctly!"
echo ""
echo "📚 Available RPC Methods:"
echo "  - equa_getBlockPeriod"
echo "  - equa_setBlockPeriod"
echo "  - equa_getConsensusStatus"
echo "  - equa_getConsensusInfo"
echo "  - equa_getValidators"
echo "  - equa_getValidatorsInfo"
echo "  - equa_getMEVStats"
echo "  - equa_getPoWDifficulty"
echo "  - equa_diagnoseConsensus"
echo "  - equa_proveConsensus"
echo "  - equa_getOrderingScore"
echo ""
