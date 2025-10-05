#!/bin/bash
# Inicia rede EQUA completa com Beacon Mock (Consensus Layer próprio)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"
ROOT_DIR="$(dirname "$DOCKER_DIR")"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   🚀 EQUA Network + Consensus Layer${NC}"
echo -e "${BLUE}========================================${NC}\n"

cd "$DOCKER_DIR"

# 1. Build beacon-mock
echo -e "${YELLOW}🔨 Building beacon-mock...${NC}"
cd "$ROOT_DIR"
docker build -f docker/Dockerfile.beacon-mock -t equa-beacon-mock:latest .

cd "$DOCKER_DIR"

# 2. Iniciar bootnode
echo -e "${YELLOW}📡 Iniciando bootnode...${NC}"
docker-compose up -d bootnode
sleep 5

# 3. Iniciar validadores (execution layer)
echo -e "${YELLOW}⚙️  Iniciando validadores (Geth)...${NC}"
docker-compose up -d validator1 validator2 validator3 validator4 validator5
sleep 10

echo -e "${GREEN}✅ Validadores iniciados!${NC}\n"

# 4. Aguardar JWT secrets serem criados
echo -e "${YELLOW}🔐 Aguardando JWT secrets...${NC}"
for i in {1..30}; do
    if docker exec equa-validator1 test -f /data/geth/jwtsecret 2>/dev/null; then
        echo -e "${GREEN}✅ JWT secrets prontos!${NC}\n"
        break
    fi
    sleep 1
done

# 5. Iniciar beacon mocks (consensus layer) - um para cada validador
echo -e "${YELLOW}🔷 Iniciando EQUA Consensus Layer (5 Beacon Mocks)...${NC}"
docker-compose up -d beacon1 beacon2 beacon3 beacon4 beacon5

sleep 5

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✅ Rede EQUA completa iniciada!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${BLUE}📊 Status dos serviços:${NC}"
docker-compose ps

echo -e "\n${YELLOW}🔍 Verificando produção de blocos...${NC}"
sleep 10

# Verificar blocos
BLOCK=$(docker exec equa-validator1 geth --exec 'eth.blockNumber' attach /data/geth.ipc 2>/dev/null || echo "0")
echo -e "  📦 Bloco atual: ${GREEN}$BLOCK${NC}"

if [ "$BLOCK" -gt "0" ]; then
    echo -e "\n${GREEN}🎉 Blocos sendo produzidos!${NC}"
else
    echo -e "\n${YELLOW}⏳ Aguardando primeiro bloco... (pode levar até 15s)${NC}"
fi

echo -e "\n${YELLOW}📝 Comandos úteis:${NC}"
echo -e "  ${BLUE}# Ver logs do beacon${NC}"
echo -e "  docker logs -f equa-beacon1\n"

echo -e "  ${BLUE}# Ver logs do validator1${NC}"
echo -e "  docker logs -f equa-validator1\n"

echo -e "  ${BLUE}# Verificar blocos${NC}"
echo -e "  docker exec equa-validator1 geth --exec 'eth.blockNumber' attach /data/geth.ipc\n"

echo -e "${GREEN}🎯 EQUA Consensus Layer ativo! Blocos serão produzidos a cada 6s${NC}\n"
