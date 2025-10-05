#!/bin/bash
# Script para iniciar um nó EQUA

set -e

# Cores para output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   Iniciando Nó EQUA${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Verificar se o geth foi compilado
if [ ! -f "./build/bin/geth" ]; then
    echo -e "${RED}❌ Erro: geth não encontrado!${NC}"
    echo -e "${BLUE}Por favor, compile primeiro: make geth${NC}"
    exit 1
fi

# Diretório de dados
DATA_DIR="${HOME}/.equa/node1"

# Verificar se foi inicializado
if [ ! -f "${DATA_DIR}/geth/chaindata/000001.log" ]; then
    echo -e "${YELLOW}⚠️  Nó não inicializado!${NC}"
    echo -e "${BLUE}Inicializando automaticamente...${NC}"
    ./init-node.sh
fi

# Configurações
HTTP_PORT=8545
WS_PORT=8546
P2P_PORT=30303
NETWORK_ID=3782

echo -e "${BLUE}📊 Configurações:${NC}"
echo -e "   Chain ID: ${GREEN}${NETWORK_ID}${NC}"
echo -e "   Data Dir: ${GREEN}${DATA_DIR}${NC}"
echo -e "   HTTP Port: ${GREEN}${HTTP_PORT}${NC}"
echo -e "   WS Port: ${GREEN}${WS_PORT}${NC}"
echo -e "   P2P Port: ${GREEN}${P2P_PORT}${NC}"
echo ""
echo -e "${YELLOW}🚀 Iniciando nó EQUA...${NC}"
echo ""

# Iniciar o nó
./build/bin/geth \
  --datadir "${DATA_DIR}" \
  --networkid ${NETWORK_ID} \
  --http \
  --http.addr "0.0.0.0" \
  --http.port ${HTTP_PORT} \
  --http.api "eth,net,web3,personal,admin,debug,miner,txpool" \
  --http.corsdomain "*" \
  --ws \
  --ws.addr "0.0.0.0" \
  --ws.port ${WS_PORT} \
  --ws.api "eth,net,web3,personal,admin,debug,miner,txpool" \
  --ws.origins "*" \
  --port ${P2P_PORT} \
  --allow-insecure-unlock \
  --nodiscover \
  --maxpeers 10 \
  --verbosity 3 \
  console

