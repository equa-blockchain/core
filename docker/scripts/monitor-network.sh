#!/bin/bash
# Script para monitorar a rede EQUA

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

clear

while true; do
    clear
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}   EQUA Network Monitor${NC}"
    echo -e "${BLUE}========================================${NC}\n"

    echo -e "${CYAN}$(date)${NC}\n"

    # Status dos containers
    echo -e "${YELLOW}📦 Containers:${NC}"
    docker ps --filter "name=equa-" --format "table {{.Names}}\t{{.Status}}" | grep -v "CONTAINER\|bootnode"
    echo ""

    # Informações de cada validador
    for i in {1..5}; do
        echo -e "${BLUE}Validador ${i}:${NC}"

        RESULT=$(docker exec equa-validator${i} geth attach /data/geth.ipc --exec "JSON.stringify({block: eth.blockNumber, peers: net.peerCount, mining: eth.mining, pending: txpool.status.pending})" 2>/dev/null || echo '{"error":true}')

        if [[ "$RESULT" != *"error"* ]]; then
            BLOCK=$(echo $RESULT | jq -r '.block')
            PEERS=$(echo $RESULT | jq -r '.peers')
            MINING=$(echo $RESULT | jq -r '.mining')
            PENDING=$(echo $RESULT | jq -r '.pending')

            echo -e "  Block: ${GREEN}${BLOCK}${NC} | Peers: ${CYAN}${PEERS}${NC} | Mining: ${YELLOW}${MINING}${NC} | Pending TXs: ${CYAN}${PENDING}${NC}"
        else
            echo -e "  ${RED}Offline ou iniciando...${NC}"
        fi
    done

    echo -e "\n${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}Pressione Ctrl+C para sair${NC}"

    sleep 5
done

