#!/bin/sh
# Entrypoint script para inicializar nós com genesis EQUA

set -e

# Se não houver chaindata, inicializar com genesis
if [ ! -d "/data/geth/chaindata" ]; then
    echo "Inicializando com genesis EQUA..."
    geth init --datadir /data /genesis/genesis.json
    echo "Genesis inicializado!"
fi

# Executar geth com os argumentos passados
exec geth "$@"

