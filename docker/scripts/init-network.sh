#!/bin/bash
# Script para inicializar a rede EQUA com bootnode correto

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   EQUA Network Initialization${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Verificar se bootnode está rodando
if ! docker ps | grep -q equa-bootnode; then
    echo -e "${RED}❌ Bootnode não está rodando!${NC}"
    echo -e "${YELLOW}Execute primeiro: docker-compose up -d bootnode${NC}\n"
    exit 1
fi

echo -e "${YELLOW}⏳ Aguardando bootnode inicializar...${NC}"
sleep 3

# Obter enode do bootnode
echo -e "${BLUE}🔍 Obtendo enode do bootnode...${NC}"

BOOTNODE_ENODE=$(docker exec equa-bootnode geth attach /data/geth.ipc --exec "admin.nodeInfo.enode" 2>/dev/null | tr -d '"' | sed 's/@[^:]*/@172.28.0.10/')

if [ -z "$BOOTNODE_ENODE" ]; then
    echo -e "${RED}❌ Não foi possível obter o enode do bootnode!${NC}"
    echo -e "${YELLOW}Verificando logs do bootnode:${NC}\n"
    docker logs equa-bootnode | tail -20
    exit 1
fi

echo -e "${GREEN}✅ Enode obtido:${NC}"
echo -e "${CYAN}${BOOTNODE_ENODE}${NC}\n"

# Atualizar docker-compose.yml com o enode correto
echo -e "${YELLOW}📝 Atualizando docker-compose.yml...${NC}"

# Fazer backup
cp docker-compose.yml docker-compose.yml.bak

# Substituir placeholder pelo enode real
sed -i.tmp "s|enode://BOOTNODE_ENODE@172.28.0.10:30303|${BOOTNODE_ENODE}|g" docker-compose.yml
rm -f docker-compose.yml.tmp

echo -e "${GREEN}✅ docker-compose.yml atualizado!${NC}\n"

# Reiniciar validadores
echo -e "${YELLOW}🔄 Reiniciando validadores...${NC}\n"

docker-compose up -d validator1 validator2 validator3 validator4 validator5

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✅ Rede inicializada com sucesso!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${YELLOW}⏳ Aguardando validadores conectarem...${NC}"
sleep 5

echo -e "\n${BLUE}📊 Status dos validadores:${NC}\n"

for i in {1..5}; do
    if docker ps | grep -q "equa-validator${i}"; then
        echo -e "  Validator ${i}: ${GREEN}✅ Running${NC}"
    else
        echo -e "  Validator ${i}: ${RED}❌ Stopped${NC}"
    fi
done

echo -e "\n${YELLOW}💡 Próximos passos:${NC}"
echo -e "  1. ${CYAN}docker logs -f equa-validator1${NC} - Ver logs"
echo -e "  2. ${CYAN}./scripts/monitor-network.sh${NC} - Monitorar rede"
echo -e "  3. ${CYAN}./scripts/register-validators.sh${NC} - Registrar validadores\n"

