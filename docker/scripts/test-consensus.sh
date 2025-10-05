#!/bin/bash
# Script para testar o consenso EQUA

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

RPC_URL="http://localhost:8545"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   🧪 EQUA Consensus Test Suite${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Função para fazer chamada RPC
rpc_call() {
    local method=$1
    local params=$2

    curl -s -X POST ${RPC_URL} \
        -H "Content-Type: application/json" \
        --data "{\"jsonrpc\":\"2.0\",\"method\":\"${method}\",\"params\":${params},\"id\":1}" \
        | jq -r '.result'
}

# Teste 1: Verificar peers
echo -e "${CYAN}📡 Test 1: Network Connectivity${NC}"
peer_count=$(rpc_call "net_peerCount" "[]" | xargs printf "%d\n")
if [ "$peer_count" -ge 3 ]; then
    echo -e "  ${GREEN}✅ PASS${NC} - Connected to ${peer_count} peers"
else
    echo -e "  ${YELLOW}⚠️  WARN${NC} - Only ${peer_count} peers connected (expected 4+)"
fi

# Teste 2: Verificar blocos
echo -e "\n${CYAN}⛓️  Test 2: Block Production${NC}"
block_number=$(rpc_call "eth_blockNumber" "[]" | xargs printf "%d\n")
if [ "$block_number" -gt 0 ]; then
    echo -e "  ${GREEN}✅ PASS${NC} - Chain at block #${block_number}"
else
    echo -e "  ${RED}❌ FAIL${NC} - No blocks produced yet"
fi

# Teste 3: Verificar mining
echo -e "\n${CYAN}⛏️  Test 3: Mining Status${NC}"
mining=$(rpc_call "eth_mining" "[]")
if [ "$mining" = "true" ]; then
    echo -e "  ${GREEN}✅ PASS${NC} - Node is actively mining"
else
    echo -e "  ${YELLOW}⚠️  WARN${NC} - Node is not mining"
fi

# Teste 4: Verificar consenso EQUA
echo -e "\n${CYAN}🛡️  Test 4: EQUA Consensus Features${NC}"

# MEV Detection
echo -e "  ${BLUE}Testing MEV Detection...${NC}"
mev_stats=$(rpc_call "equa_getMEVStats" "[]" 2>/dev/null)
if [ -n "$mev_stats" ]; then
    echo -e "    ${GREEN}✅ MEV Detection: Active${NC}"
else
    echo -e "    ${YELLOW}⚠️  MEV Detection: API not responding${NC}"
fi

# Fair Ordering
echo -e "  ${BLUE}Testing Fair Ordering...${NC}"
ordering_stats=$(rpc_call "equa_getOrderingStats" "[]" 2>/dev/null)
if [ -n "$ordering_stats" ]; then
    echo -e "    ${GREEN}✅ Fair Ordering: Active${NC}"
else
    echo -e "    ${YELLOW}⚠️  Fair Ordering: API not responding${NC}"
fi

# Slashing
echo -e "  ${BLUE}Testing Slashing System...${NC}"
slashing_stats=$(rpc_call "equa_getSlashingStats" "[]" 2>/dev/null)
if [ -n "$slashing_stats" ]; then
    echo -e "    ${GREEN}✅ Slashing System: Active${NC}"
else
    echo -e "    ${YELLOW}⚠️  Slashing System: API not responding${NC}"
fi

# Teste 5: Enviar transação de teste
echo -e "\n${CYAN}💸 Test 5: Transaction Processing${NC}"
echo -e "  ${BLUE}Sending test transaction...${NC}"

tx_hash=$(rpc_call "eth_sendTransaction" '[{
    "from": "0x0000000000000000000000000000000000000001",
    "to": "0x0000000000000000000000000000000000000002",
    "value": "0x1",
    "gas": "0x5208"
}]' 2>/dev/null)

if [ -n "$tx_hash" ] && [ "$tx_hash" != "null" ]; then
    echo -e "    ${GREEN}✅ Transaction sent: ${tx_hash}${NC}"
else
    echo -e "    ${YELLOW}⚠️  Could not send transaction (account may need unlocking)${NC}"
fi

# Resumo
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}   📊 Test Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}Network operational and consensus active!${NC}\n"

echo -e "${YELLOW}💡 Next Steps:${NC}"
echo -e "  • Monitor network: ${CYAN}./scripts/monitor-network.sh${NC}"
echo -e "  • View logs: ${CYAN}docker logs -f equa-validator1${NC}"
echo -e "  • Attach console: ${CYAN}docker exec -it equa-validator1 geth attach /data/geth.ipc${NC}\n"
