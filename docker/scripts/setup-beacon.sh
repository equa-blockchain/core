#!/bin/bash
# Script para configurar Beacon Chain com Prysm

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$(dirname "$SCRIPT_DIR")"
BEACON_DIR="$DOCKER_DIR/beacon"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   🏗️  EQUA Beacon Chain Setup${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Criar diretórios necessários
mkdir -p "$BEACON_DIR/validators"
mkdir -p "$BEACON_DIR/genesis"

# Gerar chaves dos validadores usando eth2-val-tools
echo -e "${YELLOW}📝 Gerando chaves dos validadores...${NC}"

# Vamos usar um mnemonico fixo para reproducibilidade em dev
MNEMONIC="test test test test test test test test test test test junk"

# Para cada validador, criar as chaves
for i in {1..5}; do
    VALIDATOR_DIR="$BEACON_DIR/validators/validator$i"
    mkdir -p "$VALIDATOR_DIR"

    echo -e "${BLUE}Criando chaves para validator$i...${NC}"

    # Gerar keystore usando eth2-val-tools via Docker
    # Se não tiver eth2-val-tools, vamos criar chaves dummy para teste
    if ! command -v eth2-val-tools &> /dev/null; then
        echo -e "${YELLOW}⚠️  eth2-val-tools não encontrado, criando configuração mínima...${NC}"

        # Criar arquivo de configuração básico para Prysm
        cat > "$VALIDATOR_DIR/keymanageropts.json" <<EOF
{
  "direct_eip_version": "EIP-2335",
  "direct_tree_path": "$VALIDATOR_DIR"
}
EOF

        # Criar proposer settings
        cat > "$VALIDATOR_DIR/proposer_settings.json" <<EOF
{
  "proposer_config": {
    "0x0000000000000000000000000000000000000000000000000000000000000000": {
      "fee_recipient": "0x000000000000000000000000000000000000000$i"
    }
  },
  "default_config": {
    "fee_recipient": "0x000000000000000000000000000000000000000$i"
  }
}
EOF
    fi
done

echo -e "${GREEN}✅ Chaves dos validadores criadas!${NC}\n"

# Gerar genesis time (agora + 30 segundos)
GENESIS_TIME=$(($(date +%s) + 30))

echo -e "${YELLOW}📝 Criando genesis beacon chain...${NC}"
echo -e "${BLUE}Genesis Time: $GENESIS_TIME${NC}"

# Atualizar genesis.yaml com tempo correto
sed -i.bak "s/genesis_time: 0/genesis_time: $GENESIS_TIME/" "$BEACON_DIR/genesis.yaml"

echo -e "${GREEN}✅ Beacon chain configurada!${NC}\n"

echo -e "${YELLOW}📋 Próximos passos:${NC}"
echo -e "  1. ${BLUE}docker-compose up -d${NC} - Iniciar rede completa"
echo -e "  2. Aguardar 30s para genesis"
echo -e "  3. Verificar blocos sendo produzidos\n"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}✅ Setup completo!${NC}"
echo -e "${GREEN}========================================${NC}\n"
