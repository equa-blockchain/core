#!/bin/bash
# Inicia rede EQUA em modo desenvolvimento (sem Beacon Chain)
# Os validadores minerarão blocos automaticamente usando apenas EQUA consensus

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   🚀 EQUA Network - Dev Mode${NC}"
echo -e "${BLUE}========================================${NC}\n"

cd "$DOCKER_DIR"

# Parar beacons se estiverem rodando
echo -e "${YELLOW}🛑 Parando Beacon Nodes (não necessários em dev mode)...${NC}"
docker-compose down beacon1 beacon2 beacon3 beacon4 beacon5 2>/dev/null || true

# Verificar se validadores já estão rodando
if docker ps | grep -q equa-validator; then
    echo -e "${GREEN}✅ Validadores já estão rodando!${NC}\n"
else
    # 1. Iniciar bootnode
    echo -e "${YELLOW}📡 Iniciando bootnode...${NC}"
    docker-compose up -d bootnode
    sleep 5

    # 2. Iniciar validadores
    echo -e "${YELLOW}⚙️  Iniciando validadores EQUA...${NC}"
    docker-compose up -d validator1 validator2 validator3 validator4 validator5
    sleep 10
fi

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✅ Rede EQUA ativa (modo dev)!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${BLUE}📊 Status dos validadores:${NC}"
docker-compose ps validator1 validator2 validator3 validator4 validator5

echo -e "\n${YELLOW}🔍 Verificando produção de blocos...${NC}"
sleep 3

# Verificar peers conectados
PEERS=$(docker exec equa-validator1 geth --exec 'admin.peers.length' attach /data/geth.ipc 2>/dev/null || echo "?")
echo -e "  📡 Peers conectados: ${GREEN}$PEERS${NC}"

# Verificar blocos
BLOCK=$(docker exec equa-validator1 geth --exec 'eth.blockNumber' attach /data/geth.ipc 2>/dev/null || echo "0")
echo -e "  📦 Bloco atual: ${GREEN}$BLOCK${NC}"

if [ "$BLOCK" -gt "0" ]; then
    echo -e "\n${GREEN}🎉 Blocos sendo minerados!${NC}"
else
    echo -e "\n${YELLOW}⏳ Aguardando blocos... (pode levar até 30s)${NC}"
    echo -e "${YELLOW}   Nota: Modo post-merge requer Engine API calls para minerar${NC}"
    echo -e "${YELLOW}   Para forçar mineração, use: ${BLUE}./scripts/trigger-mining.sh${NC}"
fi

echo -e "\n${YELLOW}📝 Comandos úteis:${NC}"
echo -e "  ${BLUE}# Ver logs${NC}"
echo -e "  docker logs -f equa-validator1\n"

echo -e "  ${BLUE}# Verificar blocos${NC}"
echo -e "  docker exec equa-validator1 geth --exec 'eth.blockNumber' attach /data/geth.ipc\n"

echo -e "  ${BLUE}# Enviar transação teste${NC}"
echo -e "  docker exec equa-validator1 geth --exec 'eth.sendTransaction({from:\"0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266\", to:\"0x70997970C51812dc3A010C7d01b50e0d17dc79C8\", value: web3.toWei(1, \"ether\")})' attach /data/geth.ipc\n"

echo -e "  ${BLUE}# Parar rede${NC}"
echo -e "  docker-compose down\n"

echo -e "${BLUE}📚 Documentação: docker/README.md${NC}\n"
