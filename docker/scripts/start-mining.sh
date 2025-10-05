#!/bin/bash
# Script para iniciar mineração em todos os validadores

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   Iniciando Mineração${NC}"
echo -e "${BLUE}========================================${NC}\n"

for i in {1..5}; do
    echo -e "${BLUE}Validador ${i}:${NC}"

    ADDR=$(cat ../validator/validator${i}/address.txt)

    docker exec equa-validator${i} geth attach /data/geth.ipc << EOF >/dev/null 2>&1
miner.setEtherbase("${ADDR}");
miner.start();
exit;
EOF

    echo -e "  ${GREEN}✅ Mineração iniciada${NC}"
done

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✅ Rede EQUA em produção!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${YELLOW}📊 Monitorar:${NC}"
echo -e "  docker logs -f equa-validator1"
echo -e "  docker exec -it equa-validator1 geth attach /data/geth.ipc\n"

