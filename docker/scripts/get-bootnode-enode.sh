#!/bin/bash
# Script para obter o enode do bootnode dinamicamente

BOOTNODE_CONTAINER="equa-bootnode"
MAX_RETRIES=10
RETRY_DELAY=2

echo "🔍 Obtendo enode do bootnode..."

for i in $(seq 1 $MAX_RETRIES); do
    if docker ps | grep -q "$BOOTNODE_CONTAINER"; then
        # Tentar obter enode
        ENODE=$(docker exec $BOOTNODE_CONTAINER geth attach --exec "admin.nodeInfo.enode" /data/geth.ipc 2>/dev/null | tr -d '"')

        if [ -n "$ENODE" ] && [ "$ENODE" != "null" ]; then
            # Substituir 127.0.0.1 pelo IP correto da rede Docker
            ENODE_FIXED=$(echo "$ENODE" | sed 's/@127.0.0.1/@172.25.0.10/g' | sed 's/@\[::\]/@172.25.0.10/g')
            echo "✅ Enode obtido:"
            echo "$ENODE_FIXED"
            exit 0
        fi
    fi

    echo "⏳ Tentativa $i/$MAX_RETRIES... aguardando $RETRY_DELAY segundos"
    sleep $RETRY_DELAY
done

echo "❌ Não foi possível obter o enode do bootnode após $MAX_RETRIES tentativas"
exit 1
