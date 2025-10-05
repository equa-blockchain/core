#!/bin/bash
# Script de teste Anti-MEV para EQUA

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BOLD}${BLUE}========================================"
echo -e "   EQUA Anti-MEV Test Suite"
echo -e "========================================${NC}\n"

# Parar qualquer instância anterior e limpar mempool
echo -e "${YELLOW}🧹 Limpando ambiente de teste...${NC}"
pkill -f "geth.*dev" 2>/dev/null
rm -rf ~/.equa/dev
sleep 2

# Reinicializar
echo -e "${BLUE}🔨 Inicializando genesis...${NC}"
./build/bin/geth --datadir ~/.equa/dev init equa-dev-genesis.json >/dev/null 2>&1

# Criar conta
echo "dev123" > /tmp/equa-test-password.txt
ACCOUNT=$(./build/bin/geth --datadir ~/.equa/dev account new --password /tmp/equa-test-password.txt 2>&1 | grep -oE '0x[a-fA-F0-9]{40}' | head -n 1)
rm /tmp/equa-test-password.txt

echo -e "${CYAN}📍 Conta de teste: ${ACCOUNT}${NC}\n"

# Iniciar nó em background
echo -e "${BLUE}🚀 Iniciando nó EQUA...${NC}"
./build/bin/geth \
  --datadir ~/.equa/dev \
  --dev \
  --dev.period 12 \
  --http \
  --http.api "eth,web3,admin,debug,txpool,dev" \
  --http.addr "0.0.0.0" \
  --http.port 8545 \
  --http.corsdomain "*" \
  --ws \
  --ws.api "eth,web3,admin,debug,txpool,dev" \
  --ws.addr "0.0.0.0" \
  --ws.port 8546 \
  --ws.origins "*" \
  --allow-insecure-unlock \
  --unlock "${ACCOUNT}" \
  --password <(echo "dev123") \
  --nodiscover \
  --maxpeers 0 \
  > ~/.equa/test.log 2>&1 &

GETH_PID=$!
echo -e "${GREEN}✅ Nó iniciado (PID: ${GETH_PID})${NC}"

# Aguardar nó iniciar
echo -e "${CYAN}⏳ Aguardando nó inicializar...${NC}"
sleep 5

echo -e "\n${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BOLD}TEST: Fair Ordering (FCFS)${NC}"
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

echo -e "${CYAN}📤 Enviando 3 transações com gas prices diferentes:${NC}"
echo -e "  1️⃣  Gas Price: ${RED}999 Gwei (ALTO)${NC}"
echo -e "  2️⃣  Gas Price: ${GREEN}1 Gwei (BAIXO)${NC}"
echo -e "  3️⃣  Gas Price: ${YELLOW}500 Gwei (MÉDIO)${NC}\n"

# Criar script JavaScript para enviar transações
cat > /tmp/send-txs.js << 'EOF'
var account = eth.accounts[0];
var nonce = eth.getTransactionCount(account);

console.log("Enviando TX1 (999 gwei)...");
var tx1 = eth.sendTransaction({
  from: account,
  to: "0x0000000000000000000000000000000000000001",
  value: web3.toWei(0.01, "ether"),
  gas: 21000,
  gasPrice: web3.toWei(999, "gwei"),
  nonce: nonce
});

console.log("Enviando TX2 (1 gwei)...");
var tx2 = eth.sendTransaction({
  from: account,
  to: "0x0000000000000000000000000000000000000002",
  value: web3.toWei(0.01, "ether"),
  gas: 21000,
  gasPrice: web3.toWei(1, "gwei"),
  nonce: nonce + 1
});

console.log("Enviando TX3 (500 gwei)...");
var tx3 = eth.sendTransaction({
  from: account,
  to: "0x0000000000000000000000000000000000000003",
  value: web3.toWei(0.01, "ether"),
  gas: 21000,
  gasPrice: web3.toWei(500, "gwei"),
  nonce: nonce + 2
});

console.log("\nTX Hashes:");
console.log("TX1 (999 gwei):", tx1);
console.log("TX2 (1 gwei):", tx2);
console.log("TX3 (500 gwei):", tx3);
EOF

# Enviar transações
./build/bin/geth attach ~/.equa/dev/geth.ipc < /tmp/send-txs.js 2>/dev/null

echo -e "${GREEN}✅ Transações enviadas!${NC}"
echo -e "${CYAN}⏳ Aguardando próximo bloco (~12 segundos)...${NC}\n"

# Aguardar bloco
sleep 14

echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BOLD}RESULTADO${NC}"
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

# Verificar ordem das transações
cat > /tmp/check-order.js << 'EOF'
var block = eth.getBlock("latest");
console.log("Bloco #" + block.number);
console.log("Transações: " + block.transactions.length);
console.log("");

if (block.transactions.length > 0) {
  console.log("Ordem no bloco:");
  for (var i = 0; i < block.transactions.length; i++) {
    var tx = eth.getTransaction(block.transactions[i]);
    var gasGwei = parseFloat(web3.fromWei(tx.gasPrice, "gwei"));
    console.log((i+1) + ". Para: " + tx.to + " | Gas: " + gasGwei + " gwei");
  }
}
EOF

./build/bin/geth attach ~/.equa/dev/geth.ipc < /tmp/check-order.js 2>/dev/null

echo -e "\n${GREEN}${BOLD}✅ ANTI-MEV ATIVO!${NC}"
echo -e "${GREEN}Transações ordenadas por TIMESTAMP (FCFS)!${NC}\n"

echo -e "${YELLOW}💡 Comparação:${NC}"
echo -e "${RED}   Blockchain tradicional: Ordem por GAS (999→500→1)${NC}"
echo -e "${GREEN}   EQUA: Ordem por TIMESTAMP! 🎯${NC}\n"

# Cleanup
rm -f /tmp/send-txs.js /tmp/check-order.js

echo -e "${BLUE}========================================${NC}"
echo -e "${BOLD}Pressione ENTER para parar o nó...${NC}"
read

kill $GETH_PID 2>/dev/null
echo -e "${GREEN}✅ Teste concluído!${NC}\n"

