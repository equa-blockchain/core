#!/bin/bash
# Script para inicializar um nó EQUA

set -e

# Cores para output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   Inicializando Nó EQUA${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Verificar se o geth foi compilado
if [ ! -f "./build/bin/geth" ]; then
    echo -e "${RED}❌ Erro: geth não encontrado!${NC}"
    echo -e "${BLUE}Por favor, compile primeiro: make geth${NC}"
    exit 1
fi

# Criar diretório de dados
DATA_DIR="${HOME}/.equa/node1"
echo -e "${BLUE}📁 Criando diretório de dados: ${DATA_DIR}${NC}"
mkdir -p "${DATA_DIR}"

# Verificar se já foi inicializado
if [ -f "${DATA_DIR}/geth/chaindata/000001.log" ]; then
    echo -e "${GREEN}✅ Nó já inicializado!${NC}"
    echo -e "${BLUE}Para resetar, delete: ${DATA_DIR}${NC}"
    exit 0
fi

# Inicializar com genesis
echo -e "${BLUE}🔨 Inicializando blockchain com genesis...${NC}"
./build/bin/geth --datadir "${DATA_DIR}" init equa-genesis.json

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}   ✅ Nó inicializado com sucesso!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${BLUE}Para iniciar o nó, execute:${NC}"
echo -e "${GREEN}  ./start-node.sh${NC}"
echo ""

