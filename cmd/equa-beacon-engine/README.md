# EQUA Beacon Mock - Consensus Layer

Implementação simplificada de Consensus Layer para EQUA Network que substitui beacon clients tradicionais (Prysm, Lighthouse, etc).

## 🎯 Objetivo

O **beacon-mock** atua como Consensus Layer minimalista que:
- Comunica com Geth via **Engine API** (post-merge)
- Dispara produção de blocos em intervalos regulares
- Permite que o **consenso EQUA** (no Geth) faça toda a lógica de validação
- Substitui a necessidade de beacon clients completos em ambientes dev/testnet

## 🏗️ Arquitetura

```
┌─────────────────┐
│  beacon-mock    │ (Este serviço)
│  (Go)           │
└────────┬────────┘
         │ Engine API
         │ (JWT Auth)
         ▼
┌─────────────────┐
│  Geth EQUA      │
│  (Execution)    │
└─────────────────┘
         │
         ▼
   EQUA Consensus
   - MEV Detection
   - Fair Ordering
   - PoW + PoS
   - Slashing
```

## 🚀 Como Funciona

1. **Inicialização**: Conecta ao Geth via Engine API (porta 8551)
2. **Loop de Produção**: A cada X segundos (default: 6s):
   - Chama `engine_forkchoiceUpdatedV3` com novo payload
   - Aguarda Geth construir o bloco
   - Chama `engine_getPayloadV3` para obter bloco
   - Chama `engine_newPayloadV3` para submeter bloco
3. **EQUA Consensus**: Geth executa toda lógica EQUA internamente

## ⚙️ Configuração

### Flags

- `--execution-endpoint`: URL do Engine API (default: `http://localhost:8551`)
- `--jwt-secret`: Caminho para arquivo JWT secret
- `--block-time`: Intervalo entre blocos (default: `6s`)
- `--validator-id`: ID do validador (1-5)

### Exemplo

```bash
beacon-mock \
  --execution-endpoint=http://172.25.0.101:8551 \
  --jwt-secret=/path/to/jwtsecret \
  --block-time=6s \
  --validator-id=1
```

## 🐳 Docker

Build:
```bash
docker build -f docker/Dockerfile.beacon-mock -t equa-beacon-mock .
```

Run:
```bash
docker run --rm \
  -v validator1-data:/validator-data:ro \
  equa-beacon-mock \
  --execution-endpoint=http://172.25.0.101:8551 \
  --jwt-secret=/validator-data/geth/jwtsecret
```

## 📡 Engine API Methods

Implementado:
- ✅ `engine_forkchoiceUpdatedV3` - Atualiza head e solicita novo payload
- ✅ `engine_getPayloadV3` - Obtém payload construído
- ✅ `engine_newPayloadV3` - Submete payload para execução
- ✅ `eth_getBlockByNumber` - Obtém bloco atual

## 🔄 Diferença vs Beacon Client Real

| Feature | Beacon Client (Prysm) | beacon-mock |
|---------|----------------------|-------------|
| P2P Gossip | ✅ Sim | ❌ Não |
| Validator Keys | ✅ BLS signatures | ❌ Não necessário |
| Attestations | ✅ Sim | ❌ Não |
| Finality | ✅ Casper FFG | ⚠️ Simplificado |
| Fork Choice | ✅ LMD-GHOST | ⚠️ Sempre latest |
| Sync Committee | ✅ Sim | ❌ Não |
| **Tamanho** | ~500MB | ~20MB |
| **Complexidade** | Alta | Baixa |
| **Uso** | Produção | Dev/Testnet |

## 🎯 Quando Usar

### ✅ Use beacon-mock para:
- Desenvolvimento local
- Testnets privadas
- CI/CD testing
- Protótipos rápidos
- Ambientes onde consenso EQUA é mais importante que finality tradicional

### ❌ NÃO use para:
- Mainnet
- Redes públicas com múltiplos operadores
- Ambientes que precisam de finality real

## 🔮 Roadmap

### v1.0 (Atual)
- [x] Engine API básico
- [x] Produção de blocos por intervalo
- [x] JWT authentication

### v2.0 (Futuro)
- [ ] Integração com EQUA StakeManager para seleção de validadores
- [ ] Múltiplos beacon-mocks coordenados
- [ ] Metrics/Prometheus
- [ ] Finality checkpoints

### v3.0 (Long-term)
- [ ] P2P entre beacon-mocks
- [ ] Fork choice real
- [ ] Compatibilidade com Validator Clients padrão

## 📚 Referências

- [Engine API Spec](https://github.com/ethereum/execution-apis/tree/main/src/engine)
- [Post-Merge Architecture](https://ethereum.org/en/roadmap/merge/)
- [EQUA Consensus](../../consensus/equa/)

---

**EQUA Chain** - Simplified Consensus Layer 🔷
