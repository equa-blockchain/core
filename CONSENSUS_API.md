# 🔷 EQUA Consensus API Reference

## 📋 Visão Geral

Este documento descreve todas as APIs disponíveis para interagir com o consenso EQUA, incluindo diagnósticos, controle de tempo de bloco e prova de funcionamento.

---

## 🎯 APIs Disponíveis

### 1. **equa_getBlockPeriod**

Retorna o período de bloco configurado (tempo entre blocos em segundos).

**Chamada:**
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_getBlockPeriod","params":[],"id":1}'
```

**Resposta:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": 12
}
```

---

### 2. **equa_setBlockPeriod**

Ajusta o período de bloco dinamicamente.

**Parâmetros:**
- `period` (uint64): Novo período em segundos (mínimo: 1, máximo: 300)

**Chamada:**
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_setBlockPeriod","params":[8],"id":1}'
```

**Resposta:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "success": true,
    "oldPeriod": 12,
    "newPeriod": 8,
    "message": "Block period updated successfully"
  }
}
```

⚠️ **Nota:** O beacon-mock precisa ser reiniciado para que a mudança tenha efeito completo.

---

### 3. **equa_getConsensusStatus**

Retorna status completo do engine de consenso.

**Chamada:**
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_getConsensusStatus","params":[],"id":1}'
```

**Resposta:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "engine": "EQUA",
    "version": "1.0.0",
    "status": "active",
    "config": {
      "period": 12,
      "epoch": 7200,
      "thresholdShares": 2,
      "mevBurnPercentage": 80,
      "powDifficulty": 1000000,
      "validatorReward": "2000000000000000000",
      "slashingPercentage": 50
    },
    "validators": {
      "count": 5,
      "totalStake": "160000000000000000000",
      "active": 5
    },
    "state": {
      "currentEpoch": 0,
      "currentBlockNumber": 42
    },
    "pow": {
      "difficulty": "1000000",
      "totalAttempts": 150,
      "averageTime": "2.5s",
      "hashRate": 150000,
      "lastSolveTime": "2.3s"
    },
    "ordering": {
      "totalTransactions": 1250,
      "orderingViolations": 5,
      "averageOrderingScore": 0.98,
      "fairOrderingRate": 0.996
    }
  }
}
```

---

### 4. **equa_diagnoseConsensus**

Executa diagnóstico completo do consenso.

**Parâmetros:**
- `blockCount` (int): Número de blocos a analisar (padrão: 10, máximo: 100)

**Chamada:**
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_diagnoseConsensus","params":[10],"id":1}'
```

**Resposta:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "blockRange": {
      "from": 32,
      "to": 42
    },
    "health": {
      "status": "healthy",
      "issues": [],
      "score": 100.0
    },
    "performance": {
      "powSolveTime": "2.5s",
      "hashRate": 150000,
      "orderingRate": 0.996,
      "orderingScore": 0.98,
      "blockPeriod": 12
    },
    "mev": {
      "totalMEV": "0",
      "totalBurned": "0",
      "blocksWithMEV": 0,
      "burnPercentage": 80
    },
    "recommendations": []
  }
}
```

**Possíveis Status de Saúde:**
- `healthy`: Sem problemas
- `warning`: Problemas menores detectados (1-2 issues)
- `critical`: Problemas críticos detectados (3+ issues)

**Exemplos de Issues:**
- "No validators registered in StakeManager (using defaults)"
- "PoW solve time exceeds block period"
- "Fair ordering rate is below 95%"

---

### 5. **equa_proveConsensus**

Gera prova criptográfica de funcionamento do consenso para um bloco específico.

**Parâmetros:**
- `blockNumber` (uint64): Número do bloco

**Chamada:**
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_proveConsensus","params":[42],"id":1}'
```

**Resposta:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "blockNumber": 42,
    "blockHash": "0x1234...",
    "timestamp": 1730875200,
    "difficulty": "1000000",
    "coinbase": "0x0000000000000000000000000000000000000001",
    "powProof": {
      "valid": true,
      "nonce": 12345,
      "mixDigest": "0xabcd...",
      "difficulty": "1000000"
    },
    "stakeProof": {
      "validator": "0x0000000000000000000000000000000000000001",
      "defaultValidator": true,
      "stake": "32000000000000000000"
    },
    "orderingProof": {
      "score": 0.98,
      "violations": 0,
      "fairOrdering": true
    },
    "mevProof": {
      "mevDetected": false,
      "scanner": "active"
    },
    "proofHash": "0x5678...",
    "generated": 1730875300
  }
}
```

---

### 6. **equa_getValidators**

Lista todos os validadores ativos.

**Chamada:**
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_getValidators","params":[],"id":1}'
```

**Resposta:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": [
    {
      "address": "0x0000000000000000000000000000000000000001",
      "stake": "32000000000000000000",
      "active": true
    },
    {
      "address": "0x0000000000000000000000000000000000000002",
      "stake": "32000000000000000000",
      "active": true
    }
    // ... outros validadores
  ]
}
```

---

### 7. **equa_getMEVStats**

Retorna estatísticas de MEV detectado e queimado.

**Parâmetros:**
- `blockCount` (int): Número de blocos a analisar

**Chamada:**
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_getMEVStats","params":[10],"id":1}'
```

**Resposta:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "totalMEV": "1500000000000000000",
    "totalBurned": "1200000000000000000",
    "blocksWithMEV": 3,
    "burnPercentage": 80,
    "mevByType": {
      "sandwich": "800000000000000000",
      "frontrun": "500000000000000000",
      "arbitrage": "200000000000000000"
    },
    "averageMEVPerBlock": "500000000000000000"
  }
}
```

---

### 8. **equa_getOrderingScore**

Retorna score de qualidade da ordenação de transações de um bloco.

**Parâmetros:**
- `blockNumber` (uint64): Número do bloco

**Chamada:**
```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_getOrderingScore","params":[42],"id":1}'
```

**Resposta:**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "blockNumber": 42,
    "score": 0.98,
    "violations": 2,
    "fairOrdering": true,
    "quality": {
      "timestampOrdering": 0.99,
      "gasPriceOrdering": 0.97,
      "priorityOrdering": 1.0
    }
  }
}
```

---

## 🚀 Como Usar com o Beacon Mock

O beacon-mock agora **detecta automaticamente** o período de bloco do genesis!

### Modo Automático (Recomendado)

```bash
cd cmd/beacon-mock
go run main.go \
  --execution-endpoint=http://localhost:8551 \
  --jwt-secret=/path/to/jwt.hex \
  --validator-id=1
```

O beacon-mock vai:
1. Conectar ao RPC
2. Chamar `equa_getBlockPeriod`
3. Usar o valor do genesis automaticamente

### Modo Manual (Sobrescrever)

```bash
cd cmd/beacon-mock
go run main.go \
  --execution-endpoint=http://localhost:8551 \
  --jwt-secret=/path/to/jwt.hex \
  --validator-id=1 \
  --block-time=8s
```

---

## 🧪 Testes

Execute o script de teste completo:

```bash
cd /Users/renancorrea/Development/equa-chain
./test-consensus.sh
```

Este script testa todas as APIs e gera um relatório completo.

---

## 📊 Monitoramento em Tempo Real

### 1. Monitorar Status do Consenso

```bash
watch -n 2 'curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"equa_getConsensusStatus\",\"params\":[],\"id\":1}" | jq ".result"'
```

### 2. Monitorar Saúde do Consenso

```bash
watch -n 5 'curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"equa_diagnoseConsensus\",\"params\":[10],\"id\":1}" | jq ".result.health"'
```

### 3. Monitorar MEV

```bash
watch -n 10 'curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"equa_getMEVStats\",\"params\":[10],\"id\":1}" | jq ".result"'
```

---

## 🔧 Ajuste Dinâmico de Performance

### Exemplo: Reduzir Tempo de Bloco

```bash
# 1. Verificar período atual
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_getBlockPeriod","params":[],"id":1}'

# 2. Ajustar para 8 segundos
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_setBlockPeriod","params":[8],"id":1}'

# 3. Reiniciar beacon-mock (vai auto-detectar novo período)
```

---

## 🎯 Casos de Uso

### 1. Provar que o Consenso está Funcionando

```bash
# Obter último bloco
BLOCK=$(curl -s -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | jq -r '.result')

# Gerar prova
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d "{\"jsonrpc\":\"2.0\",\"method\":\"equa_proveConsensus\",\"params\":[$BLOCK],\"id\":1}" | jq '.result'
```

### 2. Diagnosticar Problemas de Performance

```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_diagnoseConsensus","params":[50],"id":1}' | jq '.result.recommendations'
```

### 3. Verificar se MEV está sendo Queimado

```bash
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_getMEVStats","params":[100],"id":1}' | jq '.result | {totalMEV, totalBurned, burnRate: (.totalBurned / .totalMEV * 100)}'
```

---

## ✅ Validação dos 5 Pilares

### Como Provar que os 5 Pilares Estão Funcionando

#### 1️⃣ Mempool Criptografado
```bash
# Verificar threshold encryption ativo
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_getConsensusInfo","params":[],"id":1}' | jq '.result.thresholdShares'
```

#### 2️⃣ Consensus Híbrido (PoS + PoW)
```bash
# Verificar PoW ativo e validadores
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_proveConsensus","params":[BLOCK_NUMBER],"id":1}' | jq '.result | {powProof, stakeProof}'
```

#### 3️⃣ MEV Burn Automático (80%)
```bash
# Verificar MEV queimado
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_getMEVStats","params":[100],"id":1}' | jq '.result | {totalMEV, totalBurned, burnPercentage}'
```

#### 4️⃣ Fair Ordering (FCFS)
```bash
# Verificar ordering score
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_getOrderingScore","params":[BLOCK_NUMBER],"id":1}' | jq '.result.score'
```

#### 5️⃣ Slashing Severo
```bash
# Verificar eventos de slashing
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"equa_getSlashingEvents","params":[100],"id":1}' | jq '.result'
```

---

## 📚 Referências

- [README Principal](./README-EQUA.md)
- [Implementação MEV](./MEV_IMPLEMENTATION.md)
- [Implementação Threshold](./THRESHOLD_IMPLEMENTATION.md)
- [Código Fonte Consenso](./consensus/equa/)
- [Beacon Mock](./cmd/beacon-mock/)
