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
