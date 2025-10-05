# 🔷 EQUA Beacon Engine - Implementation Guide

## 📋 Overview

O **EQUA Beacon Engine** é a camada de consenso (Consensus Layer) do EQUA Chain, responsável por:
- Seleção de proposers usando PoW+PoS híbrido
- Coleta e validação de attestations com MEV scores
- Fast finality usando threshold signatures
- Fork choice MEV-aware
- Gestão de reputation dos validadores
- Cálculo de rewards dinâmicos

## 🏗️ Arquitetura

```
┌─────────────────────────────────────────────────────────────┐
│                    EQUA Beacon Engine                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────┐         ┌──────────────┐                │
│  │  Main Engine │◄────────┤ RPC Client   │                │
│  └──────┬───────┘         └──────────────┘                │
│         │                                                   │
│    ┌────┴──────┬──────────┬──────────┬────────────┐       │
│    │           │          │          │            │       │
│  ┌─▼───┐   ┌──▼──┐   ┌───▼──┐   ┌──▼───┐   ┌────▼──┐    │
│  │ Pro │   │Attes│   │Fina- │   │Fork  │   │Reputa-│    │
│  │poser│   │tation│   │lity  │   │Choice│   │tion   │    │
│  │Sel. │   │ Pool │   │Engine│   │      │   │Manager│    │
│  └─────┘   └─────┘   └──────┘   └──────┘   └───────┘    │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                        ▲
                        │ Engine API (JWT)
                        ▼
┌─────────────────────────────────────────────────────────────┐
│              Execution Layer (Geth + EQUA)                  │
└─────────────────────────────────────────────────────────────┘
```

## 📁 Estrutura de Arquivos

```
cmd/equa-beacon-engine/
├── main.go                    # Entry point com auto-detection
└── engine/
    ├── types.go              # Core types (Validator, Attestation, etc)
    ├── config.go             # Configuration structs
    ├── proposer.go           # Hybrid PoW+PoS selection
    ├── attestation.go        # MEV-aware attestation pool
    ├── finality.go           # Fast finality engine
    ├── fork_reputation.go    # Fork choice + Reputation
    ├── rpc.go                # RPC client
    └── engine.go             # Main coordinator
```

## 🔑 Features Inovadoras

### 1. Hybrid PoW+PoS Proposer Selection

**Arquivo:** `engine/proposer.go`

```go
// Combina:
// - 70% PoS (stake weight)
// - 30% PoW (difficulty/quality)
// - VRF para unpredictability
weight = (stake * 0.7) + (powQuality * 0.3 * reputation)
```

**Vantagens:**
- PoS garante stake-weighted security
- PoW adiciona randomness e previne previsibilidade
- VRF torna seleção determinística mas não predictable
- Reputation penaliza comportamento ruim

### 2. MEV-Aware Attestations

**Arquivo:** `engine/attestation.go`

Cada attestation inclui:
```go
type Attestation struct {
    Slot          uint64
    BlockHash     common.Hash
    Validator     common.Address
    MEVScore      float64  // 0-100 (100 = sem MEV)
    OrderingScore float64  // 0-100 (100 = FCFS perfeito)
    Signature     []byte
}
```

**Como funciona:**
1. Validator atesta bloco
2. RPC call para `equa_getMEVDetected(block)` retorna MEV status
3. RPC call para `equa_getOrderingScore(block)` retorna ordering score
4. Scores são incluídos na attestation
5. Finality só acontece se scores > threshold

### 3. Fast Finality (1-2 slots)

**Arquivo:** `engine/finality.go`

```
Ethereum:         EQUA:
─────────────    ─────────
Propose           Propose
  ↓ (32 slots)      ↓ (1 slot)
Justify           Justify (if MEV < 80%)
  ↓ (32 slots)      ↓ (1 slot)
Finalize          Finalize (if ordering > 90%)
  = 64+ slots       = 2 slots
```

**Condições de Finality:**
- ✅ 2/3+ stake atestando
- ✅ MEV score médio > 80%
- ✅ Ordering score médio > 90%
- ✅ Threshold signatures válidas

### 4. MEV-Aware Fork Choice

**Arquivo:** `engine/fork_reputation.go`

```go
// Peso efetivo da fork
effectiveWeight = baseStake - mevPenalty + orderingBonus

mevPenalty = baseStake * 50%      // Se MEV detectado
orderingBonus = baseStake * 10%   // Se ordering justo
```

**Resultado:**
- Forks com MEV perdem 50% do peso
- Forks com ordering justo ganham 10% bonus
- Fork canônica = maior peso efetivo

### 5. Reputation System

**Arquivo:** `engine/fork_reputation.go`

```go
type Reputation struct {
    MEVScore        float64  // -10 por MEV, +1 por bloco limpo
    OrderingScore   float64  // Smoothed average
    UptimeScore     float64  // Participation rate
    AttestationRate float64  // % of attestations submitted
    OverallScore    float64  // Weighted average
}

// Decai 1% por época
rep = rep * 0.99
```

**Impacto:**
- Reputation < 70% → não pode propor blocos
- Reputation alta → +10% rewards
- Reputation baixa → -50% peso no proposer selection

### 6. Dynamic Rewards

**Arquivo:** `engine/fork_reputation.go`

```go
baseReward = 2 EQUA per epoch

// Multiplicadores
if !mevDetected:      +20%  // Bloco limpo
if orderingScore>95%: +15%  // Ordering justo
if reputation > 90:   +10%  // Alta reputation

// Penalty
if mevDetected:       -50%  // MEV detectado

// Exemplo:
// Bloco perfeito: 2 * (1 + 0.2 + 0.15 + 0.1) = 2.9 EQUA
// Bloco com MEV:  2 * (1 - 0.5) = 1 EQUA
```

## 🚀 Como Funciona (Passo a Passo)

### Slot Processing

```
1. SlotTicker gera tick a cada 12s
   ↓
2. ProcessSlot(slot)
   ↓
3. ProposerSelector.SelectProposer(slot)
   - Lê PoW quality do last block
   - Gera seed = hash(slot + powQuality)
   - Calcula weights híbridos
   - VRF selection
   ↓
4. Se sou proposer:
   a. proposeBlock()
      - forkchoiceUpdatedV2 (prepare)
      - getPayloadV2 (build)
      - newPayloadV2 (execute)
      - forkchoiceUpdatedV2 (finalize)
   b. createAttestation()
      - Assess MEV score
      - Assess ordering score
      - Sign attestation
   ↓
5. AttestationCollector agrega attestations
   ↓
6. FinalityChecker verifica:
   - 2/3+ stake? ✓
   - MEV score > 80%? ✓
   - Ordering > 90%? ✓
   → FINALIZE
   ↓
7. RewardCalculator distribui rewards
   - Calcula multipliers
   - Atualiza reputation
```

## 📊 Estatísticas e Monitoring

### Engine Stats
```go
type Stats struct {
    SlotsProcessed    uint64
    BlocksProposed    uint64
    MissedSlots       uint64
    AverageSlotTime   time.Duration
    LastFinalizedEpoch uint64
    Uptime            time.Duration
}
```

### Logs Importantes

```
🔷 EQUA Beacon Engine
====================
📝 Using default validator address
✅ Slot duration detected: 12s
📝 Validators loaded: count=5 active=5
🚀 EQUA Beacon Engine starting
✅ EQUA Beacon Engine started successfully

📍 Slot slot=1 epoch=0 proposer=0x0000...01
🎯 Proposing block slot=1 validator=0x0000...01
✨ Block proposed successfully slot=1 blockNumber=1 blockHash=0xabc...

🔀 Fork choice updated newHead=0xabc... height=1 effectiveWeight=1000000
```

## 🔧 Configuration

### Parâmetros Principais

```go
// Timing
SlotDuration:  12s  // Auto-detected from genesis
SlotsPerEpoch: 32   // Same as Ethereum

// Finality (EQUA innovation)
FinalityThreshold:  0.67  // 2/3 stake
JustificationDelay: 1     // 1 slot (vs Ethereum 32)
FinalizationDelay:  2     // 2 slots (vs Ethereum 64)

// Rewards
BaseRewardPerEpoch:      2 EQUA
MEVBonusMultiplier:      0.2   // +20%
OrderingBonusMultiplier: 0.15  // +15%

// PoW Integration
PoWInfluence:  0.3   // 30%
MinPoWQuality: 1000

// Reputation
ReputationDecayRate: 0.01  // 1% per epoch
MinReputationScore:  70.0  // Minimum to propose
```

## 🐳 Docker Integration

### Build
```bash
# Via Makefile
make beacon

# Via Docker
docker build -t equa-beacon -f docker/Dockerfile.beacon .

# Via script
cd docker && ./build-all.sh
```

### Run
```bash
# Via docker-compose
cd docker
docker-compose up -d

# Manual
./build/bin/equa-beacon-engine \
  --execution-endpoint=http://localhost:8551 \
  --rpc-endpoint=http://localhost:8545 \
  --jwt-secret=/path/to/jwt.hex \
  --validator-id=1
```

## 🧪 Testing

### Via RPC
```bash
# Check consensus status
curl http://localhost:8545 -X POST -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","method":"equa_getConsensusStatus","params":[],"id":1}' | jq

# Diagnose consensus
curl http://localhost:8545 -X POST -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","method":"equa_diagnoseConsensus","params":[10],"id":1}' | jq

# Get proof
curl http://localhost:8545 -X POST -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","method":"equa_proveConsensus","params":["latest"],"id":1}' | jq
```

### Via Monitor Script
```bash
cd docker
./monitor.sh
```

## 📈 Performance

### Comparação com Ethereum

| Métrica | Ethereum 2.0 | EQUA |
|---------|-------------|------|
| **Slot Time** | 12s | 12s (auto-detect) |
| **Justification** | 32 slots | 1 slot |
| **Finalization** | 64+ slots | 2 slots |
| **Finality Time** | ~12-15 min | ~24s |
| **MEV Resistance** | Nenhuma | 6 layers + penalties |
| **Fork Choice** | LMD GHOST | MEV-aware GHOST |
| **Attestations** | Basic | MEV+Ordering scores |

### Throughput Esperado

- **Blocks/min**: 5 (12s block time)
- **Finality**: 2 slots = 24s
- **Reorganization**: Improvável (fork choice penaliza MEV)

## 🎯 Roadmap

### ✅ Implementado
- [x] Hybrid PoW+PoS proposer selection
- [x] VRF-based unpredictability
- [x] MEV-aware attestations
- [x] Fast finality (1-2 slots)
- [x] MEV-aware fork choice
- [x] Reputation system with decay
- [x] Dynamic rewards
- [x] Auto-detection de block period
- [x] Docker integration completa

### 🔜 Futuro (Opcional)
- [ ] P2P gossip network para attestations
- [ ] BLS signature aggregation
- [ ] Slashing execution automática
- [ ] Metrics exporters (Prometheus)
- [ ] Validator rotation automática
- [ ] Cross-shard attestations (se sharding)

## 🛡️ Security Considerations

### Threat Model

1. **MEV Extraction**
   - Mitigação: MEV scores em attestations + fork choice penalty
   - Resultado: Blocos com MEV perdem 50% peso + não finalizam

2. **Proposer Predictability**
   - Mitigação: VRF + PoW randomness
   - Resultado: Impossível prever proposer com >1 slot de antecedência

3. **Censorship**
   - Mitigação: Fair ordering na execution layer
   - Resultado: Reordering detectado e penalizado

4. **Nothing-at-Stake**
   - Mitigação: Slashing na execution layer
   - Resultado: Validador perde stake por double-signing

5. **Long-Range Attacks**
   - Mitigação: Finality rápida + checkpoints
   - Resultado: Chain finalizada não pode ser revertida

## 📚 Referências

- **Engine API**: [Ethereum Engine API Spec](https://github.com/ethereum/execution-apis/tree/main/src/engine)
- **Gasper**: [Ethereum PoS Consensus](https://ethereum.org/en/developers/docs/consensus-mechanisms/pos/gasper/)
- **LMD GHOST**: [Fork Choice Rule](https://ethereum.org/en/developers/docs/consensus-mechanisms/pos/gasper/#fork-choice)
- **VRF**: [Verifiable Random Functions](https://en.wikipedia.org/wiki/Verifiable_random_function)

---

**EQUA Beacon Engine** - Consensus Layer de produção para blockchain anti-MEV 🛡️
