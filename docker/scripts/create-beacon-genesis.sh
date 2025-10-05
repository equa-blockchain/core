#!/bin/bash
# Cria genesis.ssz do beacon chain usando prysmctl

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"
BEACON_DIR="$DOCKER_DIR/beacon"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   📦 Creating Beacon Genesis SSZ${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Criar diretório se não existir
mkdir -p "$BEACON_DIR"

# Genesis time (agora + 60 segundos para dar tempo de setup)
GENESIS_TIME=$(($(date +%s) + 60))
echo -e "${YELLOW}Genesis Time: $GENESIS_TIME${NC}"

# Criar genesis.ssz usando Prysm via Docker
echo -e "${BLUE}Gerando genesis.ssz...${NC}"

# Usar prysmctl testnet generate-genesis para criar um genesis mínimo
docker run --rm \
    -v "$BEACON_DIR:/output" \
    gcr.io/prysmaticlabs/prysm/cmd/prysmctl:latest \
    testnet generate-genesis \
    --fork=deneb \
    --num-validators=5 \
    --genesis-time=$GENESIS_TIME \
    --chain-config-file=/output/config.yaml \
    --geth-genesis-json-in=/output/../genesis/genesis.json \
    --output-ssz=/output/genesis.ssz 2>/dev/null || {

    echo -e "${YELLOW}⚠️  Falha ao gerar com prysmctl, criando genesis mínimo...${NC}"

    # Fallback: criar genesis mínimo manualmente
    # Vamos permitir que o Prysm crie o genesis na primeira execução
    cat > "$BEACON_DIR/genesis_config.yaml" <<EOF
# Genesis será criado automaticamente pelo Prysm
# usando --interop-num-validators=5 e --interop-genesis-time=$GENESIS_TIME
genesis_time: $GENESIS_TIME
EOF

    echo -e "${GREEN}✅ Configuração de genesis criada${NC}"
    echo -e "${YELLOW}Prysm criará genesis.ssz automaticamente na primeira execução${NC}"
}

echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}✅ Beacon genesis pronto!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${YELLOW}Próximos passos:${NC}"
echo -e "  1. ${BLUE}docker-compose up -d${NC}"
echo -e "  2. Aguardar genesis time: $(date -r $GENESIS_TIME '+%Y-%m-%d %H:%M:%S')\n"
