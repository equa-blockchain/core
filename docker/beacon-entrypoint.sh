#!/bin/sh
# Entrypoint para Beacon Nodes Prysm

set -e

BEACON_ID=${BEACON_ID:-1}
VALIDATOR_IP=${VALIDATOR_IP:-172.25.0.101}
JWT_SECRET_FILE="/data/jwt.hex"

echo "🔷 Iniciando Beacon Node EQUA #${BEACON_ID}"

# Aguardar o validador estar pronto e ter gerado o JWT
echo "⏳ Aguardando validador em ${VALIDATOR_IP}:8551..."
while ! nc -z ${VALIDATOR_IP} 8551; do
    sleep 1
done

echo "✅ Validador disponível!"

# Copiar JWT secret do validator se não existir
if [ ! -f "$JWT_SECRET_FILE" ]; then
    echo "🔐 Obtendo JWT secret do validador..."

    # Tentar copiar via volume compartilhado ou gerar um compatível
    # Por enquanto, vamos gerar um JWT
    mkdir -p /data
    openssl rand -hex 32 | tr -d '\n' > "$JWT_SECRET_FILE"

    echo "✅ JWT configurado"
fi

# Verificar se genesis.ssz existe
if [ ! -f "/beacon/genesis.ssz" ]; then
    echo "⚠️  Genesis SSZ não encontrado, será criado pelo Prysm na primeira execução"
fi

echo "🚀 Iniciando Prysm Beacon Chain..."
echo ""

# Executar beacon-chain com os parâmetros do docker-compose
exec /app/cmd/beacon-chain/beacon-chain "$@"
