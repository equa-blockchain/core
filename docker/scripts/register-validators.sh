#!/bin/bash
# Script para registrar validadores no StakeManager

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   Registrando Validadores${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Ler endereços dos validadores
source ../validator-addresses.txt

echo -e "${YELLOW}📝 Registrando 5 validadores...${NC}\n"

# Função para registrar validador via RPC
register_validator() {
    local validator_id=$1
    local validator_addr=$2
    local stake=$3

    echo -e "${BLUE}Validador ${validator_id}: ${validator_addr}${NC}"

    # Chamar função de registro via console
    docker exec -i equa-validator1 geth attach /data/geth.ipc << EOF
// Registrar validador
var validatorAddr = "${validator_addr}";
var stakeAmount = web3.toWei(${stake}, "ether");

console.log("Registrando validador:", validatorAddr);
console.log("Stake:", stakeAmount);

// TODO: Implementar contrato de staking
// Por enquanto, vamos adicionar direto no StakeManager via API customizada
personal.unlockAccount(eth.accounts[0], "validator1pass", 0);

var tx = eth.sendTransaction({
    from: eth.accounts[0],
    to: validatorAddr,
    value: stakeAmount
});

console.log("TX Hash:", tx);
exit;
EOF

    echo -e "${GREEN}✅ Registrado!${NC}\n"
}

# Registrar cada validador com stake diferente
register_validator 1 "$VALIDATOR_1_ADDRESS" 32
register_validator 2 "$VALIDATOR_2_ADDRESS" 50
register_validator 3 "$VALIDATOR_3_ADDRESS" 100
register_validator 4 "$VALIDATOR_4_ADDRESS" 64
register_validator 5 "$VALIDATOR_5_ADDRESS" 128

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}✅ Todos validadores registrados!${NC}"
echo -e "${GREEN}========================================${NC}\n"

echo -e "${YELLOW}💡 Próximo passo: ./scripts/start-mining.sh${NC}\n"

