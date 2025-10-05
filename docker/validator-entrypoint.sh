#!/bin/sh
# Entrypoint simplificado - toda lógica de consenso está no Go

set -e

VALIDATOR_ID=${VALIDATOR_ID:-1}
JWT_SECRET_FILE="/data/geth/jwtsecret"

echo "🚀 Iniciando Validador EQUA #${VALIDATOR_ID}"

# Inicializar genesis se necessário
if [ ! -d "/data/geth/chaindata" ]; then
    echo "📝 Inicializando genesis..."
    geth init --datadir /data /genesis/genesis.json
fi

# Gerar JWT secret se não existir
if [ ! -f "$JWT_SECRET_FILE" ]; then
    echo "🔐 Gerando JWT secret..."
    mkdir -p /data/geth
    openssl rand -hex 32 > "$JWT_SECRET_FILE"
fi

echo "✅ Validator EQUA #${VALIDATOR_ID} pronto!"
echo "   Consenso híbrido PoS+PoW ativo"
echo "   Fair Ordering (FCFS) habilitado"
echo "   MEV Detection & Burn habilitado"
echo ""

# Iniciar geth (todos os parâmetros vêm do docker-compose.yml)
exec geth "$@"
