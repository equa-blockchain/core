#!/bin/bash
# Script para configurar validadores na rede EQUA

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   EQUA Validator Network Setup${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Criar diretórios
mkdir -p docker/validator/{validator1,validator2,validator3,validator4,validator5}
mkdir -p docker/genesis

echo -e "${YELLOW}📝 Criando contas dos validadores...${NC}\n"

# Criar contas para cada validador
for i in {1..5}; do
    echo -e "${BLUE}Validador $i:${NC}"

    # Criar senha
    echo "validator${i}pass" > docker/validator/validator${i}/password.txt

    # Criar conta
    ACCOUNT=$(./build/bin/geth account new \
        --datadir docker/validator/validator${i} \
        --password docker/validator/validator${i}/password.txt \
        2>&1 | grep -oE '0x[a-fA-F0-9]{40}' | head -n 1)

    echo "$ACCOUNT" > docker/validator/validator${i}/address.txt
    echo -e "  ${GREEN}Address: $ACCOUNT${NC}"

    # Gerar chave BLS (simulada por enquanto - usaremos a privkey)
    echo "BLS_KEY_${i}" > docker/validator/validator${i}/bls_key.txt
    echo "BLS_PUBKEY_${i}" > docker/validator/validator${i}/bls_pubkey.txt
done

echo -e "\n${GREEN}✅ Contas criadas!${NC}\n"

# Copiar genesis
echo -e "${YELLOW}📋 Copiando genesis...${NC}"
cp docker/genesis/equa-validators-genesis.json docker/genesis/genesis.json

echo -e "${GREEN}✅ Genesis copiado!${NC}\n"

# Inicializar cada validador com genesis
echo -e "${YELLOW}🔨 Inicializando validadores com genesis...${NC}\n"

for i in {1..5}; do
    echo -e "${BLUE}Inicializando validador $i...${NC}"
    ./build/bin/geth init \
        --datadir docker/validator/validator${i} \
        docker/genesis/genesis.json \
        2>&1 | grep -E "(Successfully|Database)"
done

echo -e "\n${GREEN}✅ Validadores inicializados!${NC}\n"

# Criar arquivo com endereços dos validadores
echo -e "${YELLOW}📝 Criando registro de validadores...${NC}"

cat > docker/validator-addresses.txt << EOF
# Endereços dos Validadores EQUA
# Gerado em: $(date)

EOF

for i in {1..5}; do
    ADDR=$(cat docker/validator/validator${i}/address.txt)
    echo "VALIDATOR_${i}_ADDRESS=${ADDR}" >> docker/validator-addresses.txt
done

cat docker/validator-addresses.txt

echo -e "\n${GREEN}✅ Setup concluído!${NC}\n"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   Próximos passos:${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "1. ${YELLOW}cd docker${NC}"
echo -e "2. ${YELLOW}docker-compose up -d --build${NC}"
echo -e "3. ${YELLOW}./scripts/register-validators.sh${NC}"
echo -e "4. ${YELLOW}./scripts/start-mining.sh${NC}\n"

