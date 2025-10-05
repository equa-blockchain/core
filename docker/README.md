# 🐳 EQUA Network - Docker Setup

Rede de desenvolvimento local com 5 validadores + 5 beacon engines, executando o consenso híbrido EQUA (PoS+PoW com anti-MEV).

## 🏗️ Arquitetura

```
┌─────────────────────────────────────────────────────────┐
│                    EQUA Network                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Consensus Layer (Beacon Engines)                      │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐               │
│  │Beacon 1 │  │Beacon 2 │  │Beacon 3 │ ... (5 total) │
│  └────┬────┘  └────┬────┘  └────┬────┘               │
│       │ Engine API │            │                     │
│  ┌────▼────┐  ┌────▼────┐  ┌────▼────┐               │
│  │ Geth 1  │  │ Geth 2  │  │ Geth 3  │ ... (5 total) │
│  └─────────┘  └─────────┘  └─────────┘               │
│  Execution Layer (Validators)                         │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

## 🚀 Quick Start

### 1. Build All Components
```bash
cd docker
./build-all.sh
```

### 2. Start Network
```bash
./start-network.sh
```

### 3. Monitor Network
```bash
./monitor.sh
```

### 4. Stop Network
```bash
# Preserve data
./stop-network.sh

# Clean all data
./stop-network.sh --clean
```

## 📡 Endpoints

- **Validator 1**: http://localhost:8545 (WS: 8546)
- **Validator 2**: http://localhost:8547
- **Validator 3**: http://localhost:8548
- **Validator 4**: http://localhost:8549
- **Validator 5**: http://localhost:8550

## 🛡️ EQUA Beacon Engine Features

### Execution Layer (`/consensus/equa`)
1. **MEV Detection** - 6 camadas de proteção anti-MEV
2. **Fair Ordering** - FCFS com 6 critérios de validação
3. **Threshold Encryption** - Mempool criptografado
4. **Slashing** - 8 tipos de violações detectadas
5. **Stake Management** - Gestão de validadores com reputation

### Consensus Layer (`/cmd/equa-beacon-engine`)
1. **Hybrid PoW+PoS** - Proposer selection com VRF
2. **MEV-Aware Attestations** - Score de MEV e ordering em cada attestation
3. **Fast Finality** - Finality em 1-2 slots (vs 64+ do Ethereum)
4. **MEV-Aware Fork Choice** - Penaliza forks com MEV
5. **Reputation System** - Track de comportamento com decay
6. **Dynamic Rewards** - Bonuses para blocos limpos (+20% no MEV, +15% ordering)

## 🔍 Comandos Úteis

### Logs
```bash
# Validator logs
docker-compose logs -f validator1

# Beacon engine logs
docker-compose logs -f beacon1

# All logs
docker-compose logs -f
```

### Console Geth
```bash
# Attach to validator
docker exec -it equa-validator1 geth attach /data/geth.ipc

# Example commands
eth.blockNumber
equa.getConsensusStatus()
equa.getValidators()
equa.getMEVStats(10)
```

### Network Management
```bash
# Restart single service
docker-compose restart beacon1

# View running containers
docker-compose ps

# Clean restart
docker-compose down -v && ./start-network.sh
```

## 🐛 Troubleshooting

### Validadores não conectam
```bash
# Verifique subnet
docker network inspect docker_equa-network

# Verifique bootnode
docker logs equa-bootnode

# Verifique peers
docker exec -it equa-validator1 geth attach /data/geth.ipc --exec "admin.peers"
```

### Beacon engine não conecta
```bash
# Verifique logs do beacon
docker logs equa-beacon1

# Verifique JWT secret
docker exec equa-validator1 cat /data/geth/jwtsecret

# Teste Engine API manualmente
curl http://172.25.0.101:8551 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"engine_exchangeCapabilities","params":[],"id":1}'
```

### Sem blocos sendo produzidos
```bash
# Verifique se mineração está ativa
docker exec -it equa-validator1 geth attach /data/geth.ipc --exec "eth.mining"

# Verifique proposer selection
docker logs equa-beacon1 | grep "Proposer"

# Força mineração (dev only)
docker exec -it equa-validator1 geth attach /data/geth.ipc --exec "miner.start()"
```

### Beacon engine crashloop
```bash
# Ver erro detalhado
docker logs --tail 100 equa-beacon1

# Comum: JWT secret path incorreto
# Fix: Verificar volume mount em docker-compose.yml

# Comum: Execution layer não iniciou
# Fix: Aguardar validators iniciarem primeiro
```

## 📊 Monitoring & Diagnostics

### Check Consensus Health
```bash
# Via monitor script
./monitor.sh

# Via curl
curl http://localhost:8545 -X POST -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","method":"equa_diagnoseConsensus","params":[10],"id":1}' | jq
```

### Prove Consensus is Working
```bash
# Get cryptographic proof
curl http://localhost:8545 -X POST -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","method":"equa_proveConsensus","params":["latest"],"id":1}' | jq
```

## 🔧 Configuration

### Beacon Engine Parameters
Edite `docker-compose.yml` para ajustar:
- `--slot-duration`: Auto-detectado do genesis (padrão: 12s)
- `--slots-per-epoch`: Slots por época (padrão: 32)
- `--pow-influence`: PoW influence % (padrão: 0.3 = 30%)
- `--mev-bonus`: Bonus por bloco sem MEV (padrão: 0.2 = 20%)
- `--ordering-bonus`: Bonus por ordering justo (padrão: 0.15 = 15%)
- `--min-reputation`: Score mínimo para propor (padrão: 70.0)

### Validator Parameters
Edite genesis: `/Users/renancorrea/Development/equa-chain/equa-genesis.json`
- `period`: Block time em segundos
- Atualize via API: `equa_setBlockPeriod`

---

**EQUA Chain** - Blockchain justa e resistente a MEV 🛡️

## 🎯 Próximos Passos

✅ **Implementado**:
- Hybrid PoW+PoS proposer selection
- MEV-aware attestations
- Fast finality (1-2 slots)
- MEV-aware fork choice
- Reputation system with decay
- Dynamic rewards

🔜 **Futuro** (opcional):
- P2P gossip para attestations
- Validator key management (BLS)
- Slashing execution automática
- Metrics exporters (Prometheus)
- Block explorer integration
