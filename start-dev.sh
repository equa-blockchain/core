#!/bin/bash
# Script para iniciar EQUA em modo desenvolvimento com mineração

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   EQUA - Modo Desenvolvimento${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

if [ ! -f "./build/bin/geth" ]; then
    echo -e "${RED}❌ Erro: geth não encontrado!${NC}"
    exit 1
fi

# Limpar dados antigos
echo -e "${YELLOW}🧹 Limpando dados antigos...${NC}"
rm -rf ~/.equa/dev

# Reinicializar com genesis dev
echo -e "${BLUE}🔨 Inicializando com genesis dev...${NC}"
mkdir -p ~/.equa/dev
./build/bin/geth --datadir ~/.equa/dev init equa-dev-genesis.json

# Criar conta
echo -e "${BLUE}📝 Criando conta de mineração...${NC}"
echo "dev123" > /tmp/equa-dev-password.txt
MINER_ACCOUNT=$(./build/bin/geth --datadir ~/.equa/dev account new --password /tmp/equa-dev-password.txt 2>&1 | grep -oE '0x[a-fA-F0-9]{40}' | head -n 1)
rm /tmp/equa-dev-password.txt

echo -e "${GREEN}✅ Conta criada: ${MINER_ACCOUNT}${NC}"
echo ""
echo -e "${BLUE}📊 Configurações:${NC}"
echo -e "   Chain ID: ${GREEN}3782${NC}"
echo -e "   Miner: ${GREEN}${MINER_ACCOUNT}${NC}"
echo -e "   HTTP: ${GREEN}http://localhost:8545${NC}"
echo ""
echo -e "${YELLOW}🚀 Iniciando EQUA em modo dev...${NC}"
echo ""

# Criar arquivo de senha
echo "dev123" > /tmp/equa-dev-password.txt

# Iniciar em modo dev com auto-mining
./build/bin/geth \
  --datadir ~/.equa/dev \
  --http \
  --http.addr "0.0.0.0" \
  --http.port 8545 \
  --http.api "eth,net,web3,admin,debug,miner,txpool,personal" \
  --http.corsdomain "*" \
  --ws \
  --ws.addr "0.0.0.0" \
  --ws.port 8546 \
  --ws.api "eth,net,web3,admin,debug,miner,txpool,personal" \
  --ws.origins "*" \
  --nodiscover \
  --maxpeers 0 \
  --verbosity 3 \
  --dev \
  --dev.period 12 \
  --miner.etherbase "${MINER_ACCOUNT}" \
  --unlock "${MINER_ACCOUNT}" \
  --password /tmp/equa-dev-password.txt \
  --allow-insecure-unlock \
  console

# Limpar
rm -f /tmp/equa-dev-password.txt

