# 🌟 EQUA - Blockchain Anti-MEV  Equitable Quantum-resistant Universal Architecture

> Ethereum sem a taxa invisível do MEV

## 🎯 O que é EQUA?

EQUA é um fork do Ethereum que elimina **MEV (Maximum Extractable Value)** através de 5 pilares fundamentais:

1. **Mempool Criptografado** - Transações invisíveis para bots
2. **Consensus Híbrido** - PoS + Lightweight PoW para randomness
3. **MEV Burn Automático** - 80% do MEV é queimado
4. **Fair Ordering** - First-Come-First-Served (não gas wars)
5. **Slashing Severo** - Penalidades para validadores maliciosos

## 🚀 Quick Start

### 1. Compilar

```bash
make geth
```

### 2. Inicializar Nó

```bash
chmod +x init-node.sh start-node.sh
./init-node.sh
```

### 3. Iniciar Rede Local

```bash
./start-node.sh
```

O nó vai iniciar com:
- **Chain ID:** 3782
- **HTTP RPC:** http://localhost:8545
- **WebSocket:** ws://localhost:8546
- **P2P Port:** 30303

## 🔧 Comandos Úteis

### Criar Conta

```javascript
// No console do geth
personal.newAccount("sua-senha")
// Retorna: "0xabc123..."
```

### Verificar Saldo

```javascript
eth.getBalance("0xabc123...")
```

### Enviar Transação

```javascript
eth.sendTransaction({
  from: eth.accounts[0],
  to: "0xdestino...",
  value: web3.toWei(1, "ether")
})
```

### Verificar MEV Queimado

```javascript
// API customizada EQUA
equa.getMEVBurned(eth.blockNumber)
```

### Listar Validadores

```javascript
equa.getValidators()
```

## 📊 Estrutura do Consensus

```
consensus/equa/
├── equa.go           # Engine principal
├── pow.go            # PoW leve para randomness
├── stake.go          # Gestão de validadores
├── mev.go            # Detecção de MEV
├── threshold.go      # Criptografia threshold
├── ordering.go       # Fair ordering (FCFS)
├── slashing.go       # Sistema de penalidades
└── api.go            # RPC API customizada
```

## 🎨 Diferenças vs Ethereum

| Feature | Ethereum | EQUA |
|---------|----------|------|
| **MEV** | ~$520M/ano extraído | 80% queimado automaticamente |
| **Ordering** | Gas wars | First-Come-First-Served |
| **Mempool** | Público | Criptografado (threshold) |
| **Proposer** | Conhecido antecipadamente | PoW randomness |
| **Slashing** | Apenas PoS | MEV + Censura + Reordering |

## 🔥 Exemplo: Trade sem MEV

**Ethereum:**
```
1. Usuário: Comprar 10 ETH
2. Bot vê no mempool → sandwich attack
3. Usuário perde $77 em MEV
```

**EQUA:**
```
1. Usuário: Comprar 10 ETH (criptografado)
2. Bot não consegue ver → sem MEV
3. Usuário economiza $77
```

## 📈 Economia Anual

Para um trader fazendo 100 swaps/mês:

- **Ethereum:** $60,000/ano perdidos em MEV
- **EQUA:** $0 perdido em MEV

**Economia: $60,000/ano por trader ativo!**

## 🛠️ Desenvolvimento

### Executar Testes

```bash
go test ./consensus/equa/...
```

### Build Docker

```bash
docker build -t equa-chain .
```

### Compilar para Produção

```bash
make geth
# Binário em: build/bin/geth
```

## 📚 Documentação

- [Whitepaper Técnico](./docs/equa/indice.md)
- [API Reference](./docs/api.md)
- [Developer Guide](./docs/developers.md)

## 🌐 Rede

### Mainnet (Futuro)
- **Chain ID:** 3782
- **RPC:** https://rpc.equa.network
- **Explorer:** https://explorer.equa.network

### Testnet (Ativo)
- **Chain ID:** 37821
- **RPC:** https://testnet-rpc.equa.network
- **Explorer:** https://testnet.equa.network
- **Faucet:** https://faucet.equa.network

## 🤝 Contribuir

Pull requests são bem-vindos! Para mudanças grandes, abra uma issue primeiro.

## 📄 Licença

LGPL-3.0 (mesma do go-ethereum)

## 💡 Por que EQUA?

> **EQUA** = **Equ**itable (justo, sem MEV)

Ethereum cobra uma taxa invisível chamada MEV.
EQUA elimina essa taxa e torna a blockchain verdadeiramente justa.

---

**Made with ❤️ for a fairer blockchain**

