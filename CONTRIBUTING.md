# Contributing to EQUA

Thank you for your interest in contributing! 🎉

## 📋 Code of Conduct

This project adheres to our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold this code.

## 🐛 Reporting Bugs

**Before submitting:**
1. Check [existing issues](https://github.com/equa-network/equa-chain/issues)
2. Update to latest version
3. Reproduce with minimal example

**Submit via:**
- GitHub Issues: Bug report template
- Include: OS, Go version, steps to reproduce

## 💡 Proposing Features

1. Open Discussion in [GitHub Discussions](https://github.com/equa-network/equa-chain/discussions)
2. Describe use case and benefits
3. Wait for community feedback before implementing

## 🔧 Pull Request Process

### Before You Start

1. **Fork** the repository
2. **Clone** your fork
3. **Create branch:** `git checkout -b feature/amazing-feature`
4. **Setup GPG:** All commits MUST be signed

### Development
```bash
# Install dependencies
make deps

# Run tests
make test

# Lint code
make lint

# Build
make geth
