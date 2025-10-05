#!/bin/bash
# Script para iniciar a rede EQUA completa

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   🚀 EQUA Network Starter${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Limpar rede anterior se existir
echo -e "${YELLOW}🧹 Limpando containers anteriores...${NC}"
docker-compose down -v 2>/dev/null || true

echo -e "${GREEN}✅ Ambiente limpo!${NC}\n"

# Iniciar bootnode primeiro
echo -e "${BLUE}🔧 Iniciando bootnode...${NC}"
docker-compose up -d bootnode

echo -e "${YELLOW}⏳ Aguardando bootnode inicializar (10s)...${NC}"
sleep 10

# Verificar se bootnode está rodando
if ! docker ps | grep -q equa-bootnode; then
    echo -e "${RED}❌ Bootnode falhou ao iniciar!${NC}"
    echo -e "${YELLOW}Logs do bootnode:${NC}\n"
    docker logs equa-bootnode
    exit 1
fi

echo -e "${GREEN}✅ Bootnode iniciado!${NC}\n"

# Iniciar todos os validadores
echo -e "${BLUE}🔧 Iniciando validadores...${NC}"
docker-compose up -d validator1 validator2 validator3 validator4 validator5

echo -e "${YELLOW}⏳ Aguardando validadores inicializarem (15s)...${NC}"
sleep 15

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✅ Rede EQUA iniciada com sucesso!${NC}"
echo -e "${GREEN}========================================${NC}\n"

# Mostrar status
echo -e "${BLUE}📊 Status dos containers:${NC}\n"

for container in bootnode validator1 validator2 validator3 validator4 validator5; do
    if docker ps | grep -q "equa-${container}"; then
        echo -e "  ${container}: ${GREEN}✅ Running${NC}"
    else
        echo -e "  ${container}: ${RED}❌ Stopped${NC}"
    fi
done

echo -e "\n${BLUE}🌐 Endpoints RPC disponíveis:${NC}"
echo -e "  ${CYAN}Validator 1:${NC} http://localhost:8545"
echo -e "  ${CYAN}Validator 2:${NC} http://localhost:8547"
echo -e "  ${CYAN}Validator 3:${NC} http://localhost:8548"
echo -e "  ${CYAN}Validator 4:${NC} http://localhost:8549"
echo -e "  ${CYAN}Validator 5:${NC} http://localhost:8550"

echo -e "\n${BLUE}📡 WebSocket endpoints:${NC}"
echo -e "  ${CYAN}Validator 1:${NC} ws://localhost:8546"

echo -e "\n${YELLOW}💡 Comandos úteis:${NC}"
echo -e "  ${CYAN}docker logs -f equa-validator1${NC} - Ver logs do validador 1"
echo -e "  ${CYAN}docker exec -it equa-validator1 geth attach /data/geth.ipc${NC} - Console do validador 1"
echo -e "  ${CYAN}./scripts/monitor-network.sh${NC} - Monitorar rede em tempo real"
echo -e "  ${CYAN}./scripts/test-consensus.sh${NC} - Testar consenso EQUA"
echo -e "  ${CYAN}docker-compose down -v${NC} - Parar e limpar tudo\n"
