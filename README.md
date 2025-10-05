# EQUA Network 🔷

[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/equa-network/equa-chain)](https://goreportcard.com/report/github.com/equa-network/equa-chain)
[![Discord](https://img.shields.io/discord/YOUR_DISCORD_ID)](https://discord.gg/equa)
[![Twitter](https://img.shields.io/twitter/follow/equanetwork)](https://twitter.com/equanetwork)

> **Zero-MEV EVM blockchain with threshold encryption and fair ordering**

EQUA is an Ethereum-compatible blockchain that eliminates MEV (Maximal Extractable Value) through:
- 🔐 **Threshold encrypted mempool** - Transactions encrypted until block inclusion
- ⚖️ **Fair ordering** - First-come-first-served, not highest bidder
- 🔥 **MEV burn** - 80% of detected MEV is permanently burned
- 🎲 **Hybrid consensus** - PoS security + PoW randomness prevents coordination

## 🚀 Quick Start
```bash
# Clone repository
git clone https://github.com/equa-network/equa-chain.git
cd equa-chain

# Build
make geth

# Run testnet node
./build/bin/geth --testnet --http --http.api eth,net,web3,equa


 ✅ Melhorias Implementadas no Consenso Equa

  1. MEV Detection (mev.go) - Detecção Anti-MEV Avançada

  - ✨ 6 camadas de detecção: sandwich, frontrunning, arbitrage, liquidation, back-running e time-bandit attacks
  - 🔍 Detecção de oracle price updates e mudanças de liquidez
  - 📊 Análise multi-dimensional com validação cruzada
  - 🎯 Sistema de threshold adaptativo para minimizar falsos positivos

  2. Fair Ordering (ordering.go) - Ordenação Justa Multi-Dimensional

  - 🎲 Ordenação baseada em 6 critérios: MEV risk, nonce order, fairness score, priority, FCFS, gas price
  - 🛡️ Anti-sandwich: detecção e separação automática de padrões de ataque
  - 🔀 Shuffling determinístico em janelas de tempo para prevenir MEV
  - 👴 Bonus de fairness para transações antigas (anti-censorship)
  - ⚡ Validação e otimização automática de nonce ordering

  3. Light PoW (pow.go) - PoW Adaptativo e Eficiente

  - 🚀 3 estratégias de busca paralela: linear, random-jump e adaptive
  - 🧠 Worker count adaptativo baseado em dificuldade e CPU
  - ⏱️ Timeout adaptativo com base em performance histórica
  - 🎯 Sistema de quality scoring para encontrar melhores soluções
  - 💾 Cache otimizado com SHA256 para melhor performance

  4. Slashing (slashing.go) - Sistema de Penalidades Inteligente

  - 📊 8 camadas de detecção: MEV extraction, sandwich, frontrunning, arbitrage, liquidation, back-running, censorship, uncle block MEV
  - 🎚️ Sistema de severity scoring cumulativo (1-10)
  - 🔐 Geração de evidências criptográficas para cada violação
  - 📈 Detecção de censorship baseada em estatísticas de bloco
  - 🕵️ Análise de uncle block MEV extraction

  5. Stake Management (stake.go) - Gestão Avançada de Validators

  - ✅ Critérios de elegibilidade expandidos com cooldown após slashing
  - 📊 Sistema de performance scoring para validators
  - 🎯 Penalidades proporcionais baseadas em histórico
  - 🏆 Recompensas para validators com maior stake

  🎯 Benefícios Principais

  1. Segurança Máxima: 8 camadas de detecção de MEV + slashing inteligente
  2. Fairness Garantida: Ordenação multi-dimensional com anti-sandwich
  3. Performance Otimizada: PoW adaptativo com múltiplas estratégias
  4. Descentralização: Sistema de stake balanceado com scoring
  5. Auditabilidade: Evidências criptográficas de todas as violações

  Todas as implementações seguem as melhores práticas de segurança e são compatíveis com o ecossistema Ethereum/Geth! 🚀
