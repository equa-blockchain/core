# 🔷 EQUA Consensus Layer - Solução Própria

## 🎯 Visão Geral

Implementamos uma **Consensus Layer própria** para EQUA que substitui beacon clients tradicionais (Prysm, Lighthouse, etc) com uma solução customizada que:

1. **Integra com EQUA consensus** - Usa validator selection do StakeManager
2. **Round-robin inteligente** - Coordena 5 validadores para produzir blocos
3. **Engine API completo** - Comunica com Geth via protocolo post-merge
4. **Lightweight** - ~20MB vs ~500MB de beacon clients completos

## 🏗️ Arquitetura

```
┌──────────────────────────────────────────────────────┐
│              EQUA Network (Docker)                   │
│                                                      │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐    │
│  │  Beacon 1  │  │  Beacon 2  │  │  Beacon 3  │    │
│  │ (Validator │  │ (Validator │  │ (Validator │ ...│
│  │     #1)    │  │     #2)    │  │     #3)    │    │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘    │
│        │ Engine API    │ Engine API    │            │
│        ▼               ▼               ▼            │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐    │
│  │   Geth 1   │  │   Geth 2   │  │   Geth 3   │    │
│  │  (Exec)    │◄─┤  (Exec)    │◄─┤  (Exec)    │    │
│  └────────────┘  └────────────┘  └────────────┘    │
│        │               │               │            │
│        └───────────────┴───────────────┘            │
│                P2P Network                           │
│            (Sync blocks/txs)                         │
└──────────────────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│     Validator Selection Logic           │
│                                          │
│  Slot 1 → Validator #1 proposes         │
│  Slot 2 → Validator #2 proposes         │
│  Slot 3 → Validator #3 proposes         │
│  Slot 4 → Validator #4 proposes         │
│  Slot 5 → Validator #5 proposes         │
│  Slot 6 → Validator #1 proposes (loop)  │
│                                          │
│  Only the selected validator calls       │
│  Engine API to produce block             │
└─────────────────────────────────────────┘
```

## 🔄 Fluxo de Produção de Blocos

### Cada Beacon Mock (a cada 6 segundos):

1. **Incrementa slot number** - `slot++`
2. **Consulta validadores ativos**:
   - Tenta `equa_getValidators()` RPC
   - Fallback: usa lista padrão (0x...0001 a 0x...0005)
3. **Calcula proposer** - `slot % numValidators`
4. **Verifica se é sua vez**:
   - ✅ Se sim → continua para step 5
   - ❌ Se não → espera próximo slot
5. **Chama Engine API**:
   ```
   engine_forkchoiceUpdatedV3()
   → Geth prepara bloco
   engine_getPayloadV3()
   → Obtém bloco construído
   engine_newPayloadV3()
   → Submete para execução
   ```
6. **Geth executa EQUA consensus**:
   - MEV Detection
   - Fair Ordering
   - Light PoW
   - Slashing validation
   - Stake checks
7. **Bloco propagado** via P2P para outros nós

## 📊 Coordenação entre Beacons

| Slot | Time | Beacon 1 | Beacon 2 | Beacon 3 | Beacon 4 | Beacon 5 | Proposer |
|------|------|----------|----------|----------|----------|----------|----------|
| 1    | 0s   | 🎯 Propõe | ⏸️ Skip  | ⏸️ Skip  | ⏸️ Skip  | ⏸️ Skip  | **#1**   |
| 2    | 6s   | ⏸️ Skip  | 🎯 Propõe | ⏸️ Skip  | ⏸️ Skip  | ⏸️ Skip  | **#2**   |
| 3    | 12s  | ⏸️ Skip  | ⏸️ Skip  | 🎯 Propõe | ⏸️ Skip  | ⏸️ Skip  | **#3**   |
| 4    | 18s  | ⏸️ Skip  | ⏸️ Skip  | ⏸️ Skip  | 🎯 Propõe | ⏸️ Skip  | **#4**   |
| 5    | 24s  | ⏸️ Skip  | ⏸️ Skip  | ⏸️ Skip  | ⏸️ Skip  | 🎯 Propõe | **#5**   |
| 6    | 30s  | 🎯 Propõe | ⏸️ Skip  | ⏸️ Skip  | ⏸️ Skip  | ⏸️ Skip  | **#1**   |

## 🚀 Como Usar

### Iniciar rede completa:

```bash
cd /Users/renancorrea/Development/equa-chain/docker

# Build e start
./scripts/start-with-consensus.sh
```

### Monitorar:

```bash
# Logs de cada beacon
docker logs -f equa-beacon1
docker logs -f equa-beacon2
# ... etc

# Ver qual beacon está propondo
docker logs equa-beacon1 2>&1 | grep "Our turn"
docker logs equa-beacon2 2>&1 | grep "Our turn"
```

### Verificar blocos:

```bash
# Deve crescer a cada 6 segundos
docker exec equa-validator1 geth --exec 'eth.blockNumber' attach /data/geth.ipc
```

## 🎛️ Configuração

Cada beacon pode ser configurado via flags:

```yaml
command:
  - --execution-endpoint=http://172.25.0.101:8551  # Engine API (JWT)
  - --rpc-endpoint=http://172.25.0.101:8545        # HTTP RPC
  - --jwt-secret=/validator-data/geth/jwtsecret    # JWT auth
  - --block-time=6s                                # Slot duration
  - --validator-id=1                               # Validator ID
```

## 🔮 Roadmap

### ✅ v1.0 (Atual)
- [x] Engine API básico
- [x] Round-robin validator selection
- [x] Consulta lista de validadores
- [x] JWT authentication
- [x] 5 beacons coordenados

### 🔜 v2.0 (Próximo)
- [ ] Weighted selection por stake
- [ ] Integração direta com StakeManager contract
- [ ] Fallback se proposer offline
- [ ] Metrics/Prometheus
- [ ] Health checks

### 🌟 v3.0 (Futuro)
- [ ] BFT consensus entre beacons
- [ ] Finality checkpoints
- [ ] Slashing por missing blocks
- [ ] Dynamic validator set changes
- [ ] Fork choice rules

## 📚 Referências

- **Código**: `cmd/beacon-mock/main.go`
- **Dockerfile**: `docker/Dockerfile.beacon-mock`
- **Docker Compose**: `docker/docker-compose.yml`
- **EQUA Consensus**: `consensus/equa/`
- **Engine API Spec**: https://github.com/ethereum/execution-apis

## ⚠️ Limitações Atuais

| Feature | Status | Notes |
|---------|--------|-------|
| Validator selection | ✅ Round-robin | Não ponderado por stake ainda |
| Finality | ⚠️ Simplificado | Sempre latest block |
| Liveness | ⚠️ Assume todos online | Sem timeout/fallback |
| P2P gossip | ❌ Não implementado | Beacons independentes |
| Fork choice | ⚠️ Sempre longest | Sem LMD-GHOST |

## 🎯 Quando Usar vs Beacon Client Real

### ✅ Use EQUA Beacon Mock:
- **Testnet privada** - Controle total
- **Desenvolvimento local** - Setup rápido
- **CI/CD** - Testes automatizados
- **EQUA consensus é prioridade** - Foco nas features únicas

### 🔄 Migre para Beacon Real quando:
- **Mainnet** ou rede pública
- **Muitos validadores** (>10)
- **Finality crítica** - Precisa Casper FFG
- **Interoperabilidade** - Outros clientes precisam conectar

---

**EQUA Chain** - Consensus Layer Próprio 🔷
