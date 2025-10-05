#!/bin/bash
# Script para limpar completamente e reiniciar a rede EQUA

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   🧹 EQUA Network Clean Restart${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Parar todos os containers
echo -e "${YELLOW}🛑 Parando containers...${NC}"
docker-compose down

# Remover volumes
echo -e "${YELLOW}🗑️  Removendo volumes...${NC}"
docker-compose down -v

# Remover imagens antigas
echo -e "${YELLOW}🔄 Limpando imagens antigas...${NC}"
docker images | grep equa | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || true

echo -e "${GREEN}✅ Ambiente completamente limpo!${NC}\n"

# Rebuild
echo -e "${BLUE}🔨 Reconstruindo imagens...${NC}"
cd ..
docker-compose build --no-cache

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✅ Build concluído!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${YELLOW}📝 Próximo passo: Iniciar a rede${NC}"
echo -e "  ${BLUE}cd docker && ./scripts/start-network.sh${NC}\n"
