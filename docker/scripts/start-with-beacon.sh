#!/bin/bash
# Inicia rede EQUA completa com Beacon Chain (Prysm)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   🚀 EQUA Network + Beacon Chain${NC}"
echo -e "${BLUE}========================================${NC}\n"

cd "$DOCKER_DIR"

# 1. Iniciar bootnode
echo -e "${YELLOW}📡 Iniciando bootnode...${NC}"
docker-compose up -d bootnode
sleep 5

# 2. Iniciar validadores (execution layer)
echo -e "${YELLOW}⚙️  Iniciando validadores (Geth)...${NC}"
docker-compose up -d validator1 validator2 validator3 validator4 validator5
sleep 10

echo -e "${GREEN}✅ Validadores iniciados!${NC}\n"

# 3. Aguardar JWT secrets serem criados
echo -e "${YELLOW}🔐 Aguardando JWT secrets...${NC}"
for i in {1..30}; do
    if docker exec equa-validator1 test -f /data/geth/jwtsecret 2>/dev/null; then
        echo -e "${GREEN}✅ JWT secrets prontos!${NC}\n"
        break
    fi
    sleep 1
done

# 4. Iniciar beacon nodes
echo -e "${YELLOW}🔷 Iniciando Beacon Chain (Prysm)...${NC}"
docker-compose up -d beacon1 beacon2 beacon3 beacon4 beacon5

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✅ Rede EQUA completa iniciada!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${BLUE}📊 Status dos serviços:${NC}"
docker-compose ps

echo -e "\n${YELLOW}📝 Comandos úteis:${NC}"
echo -e "  ${BLUE}# Ver logs do beacon1${NC}"
echo -e "  docker logs -f equa-beacon1\n"

echo -e "  ${BLUE}# Ver logs do validator1${NC}"
echo -e "  docker logs -f equa-validator1\n"

echo -e "  ${BLUE}# Verificar blocos${NC}"
echo -e "  docker exec equa-validator1 geth --exec 'eth.blockNumber' attach /data/geth.ipc\n"

echo -e "  ${BLUE}# Monitorar rede${NC}"
echo -e "  ./scripts/monitor-network.sh\n"

echo -e "${GREEN}🎉 Aguarde ~30s para os beacon nodes sincronizarem e começarem a produzir blocos!${NC}\n"
