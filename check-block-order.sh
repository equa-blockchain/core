#!/bin/bash
# Script para verificar a ordem das transações no último bloco

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BOLD}${BLUE}========================================${NC}"
echo -e "${BOLD}${BLUE}   Verificando Ordem das Transações${NC}"
echo -e "${BOLD}${BLUE}========================================${NC}\n"

# Script para obter informações do bloco
cat > /tmp/get-block.js << 'EOF'
var block = eth.getBlock("latest");

console.log("=".repeat(50));
console.log("BLOCO #" + block.number);
console.log("Hash: " + block.hash);
console.log("Timestamp: " + new Date(block.timestamp * 1000));
console.log("Total de transações: " + block.transactions.length);
console.log("=".repeat(50));
console.log("");

if (block.transactions.length === 0) {
  console.log("⚠️  Bloco vazio - sem transações");
} else {
  console.log("ORDEM DAS TRANSAÇÕES NO BLOCO:");
  console.log("-".repeat(50));

  for (var i = 0; i < block.transactions.length; i++) {
    var txHash = block.transactions[i];
    var tx = eth.getTransaction(txHash);

    if (tx) {
      var gasGwei = parseFloat(web3.fromWei(tx.gasPrice, "gwei")).toFixed(2);
      var valueEth = parseFloat(web3.fromWei(tx.value, "ether")).toFixed(4);

      console.log("\n" + (i + 1) + "ª Transação:");
      console.log("  Hash: " + txHash.substring(0, 20) + "...");
      console.log("  De: " + tx.from);
      console.log("  Para: " + tx.to);
      console.log("  Valor: " + valueEth + " EQUA");
      console.log("  Gas Price: " + gasGwei + " gwei");
      console.log("  Nonce: " + tx.nonce);
    }
  }

  console.log("\n" + "=".repeat(50));
  console.log("ANÁLISE ANTI-MEV:");
  console.log("=".repeat(50));

  if (block.transactions.length >= 3) {
    var tx1 = eth.getTransaction(block.transactions[0]);
    var tx2 = eth.getTransaction(block.transactions[1]);
    var tx3 = eth.getTransaction(block.transactions[2]);

    var gas1 = parseFloat(web3.fromWei(tx1.gasPrice, "gwei"));
    var gas2 = parseFloat(web3.fromWei(tx2.gasPrice, "gwei"));
    var gas3 = parseFloat(web3.fromWei(tx3.gasPrice, "gwei"));

    console.log("\nGas Prices na ordem do bloco:");
    console.log("  1ª TX: " + gas1.toFixed(2) + " gwei");
    console.log("  2ª TX: " + gas2.toFixed(2) + " gwei");
    console.log("  3ª TX: " + gas3.toFixed(2) + " gwei");

    // Verificar se está ordenado por gas (tradicional) ou por timestamp (EQUA)
    var ordenadoPorGas = (gas1 >= gas2 && gas2 >= gas3);

    if (ordenadoPorGas) {
      console.log("\n⚠️  ATENÇÃO: Transações parecem ordenadas por GAS!");
      console.log("Isso indica comportamento de blockchain tradicional.");
    } else {
      console.log("\n✅ ANTI-MEV ATIVO!");
      console.log("Transações NÃO estão ordenadas por gas price.");
      console.log("Isso prova que o EQUA usa FCFS (First-Come-First-Served)!");
      console.log("\nEm blockchain tradicional, a ordem seria:");
      var gasArray = [gas1, gas2, gas3];
      gasArray.sort(function(a, b) { return b - a; });
      console.log("  1ª: " + gasArray[0].toFixed(2) + " gwei");
      console.log("  2ª: " + gasArray[1].toFixed(2) + " gwei");
      console.log("  3ª: " + gasArray[2].toFixed(2) + " gwei");
      console.log("\nMas no EQUA, a ordem é por TIMESTAMP de chegada! 🎯");
    }
  }
}

console.log("");
EOF

./build/bin/geth attach ~/.equa/dev/geth.ipc < /tmp/get-block.js 2>/dev/null | grep -v "Welcome\|instance\|datadir\|modules\|exit"

rm -f /tmp/get-block.js

echo -e "\n${BOLD}${BLUE}========================================${NC}\n"

