// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"math/big"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/params"
)

// Slasher detects malicious behavior and applies penalties
type Slasher struct {
	config *params.EquaConfig
}

// NewSlasher creates a new slasher
func NewSlasher(config *params.EquaConfig) *Slasher {
	return &Slasher{
		config: config,
	}
}

// DetectMEVExtraction detects if a validator extracted MEV
func (s *Slasher) DetectMEVExtraction(validator common.Address, txs []*types.Transaction, receipts []*types.Receipt) bool {
	// Check if validator inserted their own transactions for MEV
	for _, tx := range txs {
		if tx.From() == validator {
			// Check if this transaction appears to be MEV extraction
			if s.isMEVTransaction(tx) {
				return true
			}
		}
	}

	// Check for suspicious transaction ordering that benefits validator
	return s.detectSuspiciousOrdering(validator, txs)
}

// DetectTxReordering detects if transactions were maliciously reordered
func (s *Slasher) DetectTxReordering(txs []*types.Transaction) bool {
	if len(txs) <= 1 {
		return false
	}

	// Check for significant violations in timestamp ordering
	violations := 0
	tolerance := 5 // Allow 5% ordering violations for network latency

	for i := 1; i < len(txs); i++ {
		// Simple check: compare gas prices vs expected timestamp order
		if txs[i].GasPrice().Cmp(txs[i-1].GasPrice()) > 0 {
			// Higher gas price transaction after lower one might indicate reordering
			violations++
		}
	}

	// If more than tolerance% of transactions are out of order, flag as reordering
	return violations > (len(txs) * tolerance / 100)
}

// DetectCensorship detects if transactions were censored
func (s *Slasher) DetectCensorship(txs []*types.Transaction) bool {
	// In a real implementation, this would compare against mempool state
	// to see if high-gas transactions were deliberately excluded

	// Placeholder: check for suspicious gaps in gas prices
	if len(txs) < 2 {
		return false
	}

	// Look for large gaps in gas prices that might indicate censorship
	for i := 1; i < len(txs); i++ {
		prevGas := txs[i-1].GasPrice()
		currGas := txs[i].GasPrice()

		// If there's a large gap, it might indicate censorship
		if prevGas.Cmp(currGas) > 0 {
			gap := new(big.Int).Sub(prevGas, currGas)
			threshold := new(big.Int).Mul(currGas, big.NewInt(10)) // 10x difference

			if gap.Cmp(threshold) > 0 {
				return true
			}
		}
	}

	return false
}

// DetectValidatorCollusion detects collusion between validators
func (s *Slasher) DetectValidatorCollusion(validators []common.Address, blocks []*types.Block) bool {
	// Check for patterns indicating validator collusion
	// This is a simplified check - real implementation would be more sophisticated

	if len(blocks) < 3 {
		return false
	}

	// Check if same validators are proposing consecutive blocks (suspicious)
	consecutiveCount := 0
	for i := 1; i < len(blocks); i++ {
		if blocks[i].Header().Coinbase == blocks[i-1].Header().Coinbase {
			consecutiveCount++
		}
	}

	// If more than 50% of blocks are consecutive same validators, flag as collusion
	return consecutiveCount > len(blocks)/2
}

// isMEVTransaction checks if a transaction is likely MEV extraction
func (s *Slasher) isMEVTransaction(tx *types.Transaction) bool {
	// Check for common MEV patterns

	// Very high gas price (potential frontrunning)
	highGasThreshold := big.NewInt(1000000000000000) // 1000 gwei
	if tx.GasPrice().Cmp(highGasThreshold) > 0 {
		return true
	}

	// Check for DEX interaction (potential sandwich/arbitrage)
	if s.isDEXInteraction(tx) {
		return true
	}

	// Check for flashloan usage (potential arbitrage)
	if s.isFlashloanTransaction(tx) {
		return true
	}

	return false
}

// detectSuspiciousOrdering detects ordering that benefits a specific validator
func (s *Slasher) detectSuspiciousOrdering(validator common.Address, txs []*types.Transaction) bool {
	validatorTxCount := 0
	beneficialOrderings := 0

	for i, tx := range txs {
		if tx.From() == validator {
			validatorTxCount++

			// Check if validator's transaction is positioned to extract MEV
			if i > 0 && s.couldExtractMEV(txs[i-1], tx) {
				beneficialOrderings++
			}
		}
	}

	// If most of validator's transactions are suspiciously positioned
	return validatorTxCount > 0 && beneficialOrderings > validatorTxCount/2
}

// isDEXInteraction checks if transaction interacts with a DEX
func (s *Slasher) isDEXInteraction(tx *types.Transaction) bool {
	if tx.To() == nil || len(tx.Data()) < 4 {
		return false
	}

	// Common DEX contract addresses (simplified)
	dexAddresses := []common.Address{
		common.HexToAddress("0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D"), // Uniswap V2 Router
		common.HexToAddress("0xE592427A0AEce92De3Edee1F18E0157C05861564"), // Uniswap V3 Router
		common.HexToAddress("0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F"), // SushiSwap Router
	}

	for _, dexAddr := range dexAddresses {
		if *tx.To() == dexAddr {
			return true
		}
	}

	return false
}

// isFlashloanTransaction checks if transaction uses flashloans
func (s *Slasher) isFlashloanTransaction(tx *types.Transaction) bool {
	if len(tx.Data()) < 4 {
		return false
	}

	// Common flashloan function selectors
	flashloanSelectors := [][]byte{
		{0x5c, 0xfa, 0x42, 0xb5}, // flashLoan (Aave)
		{0xab, 0x9c, 0x4b, 0x5d}, // flashBorrow (dYdX)
	}

	selector := tx.Data()[:4]
	for _, flSelector := range flashloanSelectors {
		if string(selector) == string(flSelector) {
			return true
		}
	}

	return false
}

// couldExtractMEV checks if tx2 could extract MEV from tx1
func (s *Slasher) couldExtractMEV(tx1, tx2 *types.Transaction) bool {
	// Simplified: check if tx2 is DEX interaction following another DEX interaction
	return s.isDEXInteraction(tx1) && s.isDEXInteraction(tx2)
}

// CalculateSlashingAmount calculates how much to slash based on violation severity
func (s *Slasher) CalculateSlashingAmount(violation string, stake *big.Int) *big.Int {
	percentage := uint64(0)

	switch violation {
	case "MEV extraction":
		percentage = s.config.SlashingPercentage
	case "Transaction reordering":
		percentage = 10
	case "Transaction censorship":
		percentage = 20
	case "Validator collusion":
		percentage = 100 // Total slash for collusion
	default:
		percentage = 5 // Default minor slash
	}

	slashAmount := new(big.Int).Mul(stake, big.NewInt(int64(percentage)))
	slashAmount.Div(slashAmount, big.NewInt(100))

	return slashAmount
}