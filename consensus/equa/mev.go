// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"bytes"
	"math/big"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/params"
)

// MEVDetector detects Maximum Extractable Value (MEV) in blocks
type MEVDetector struct {
	config             *params.EquaConfig
	minProfitThreshold *big.Int
}

// NewMEVDetector creates a new MEV detector
func NewMEVDetector(config *params.EquaConfig) *MEVDetector {
	return &MEVDetector{
		config:             config,
		minProfitThreshold: big.NewInt(1e17), // 0.1 EQUA minimum profit to be considered MEV
	}
}

// DetectMEV detects and quantifies MEV in a block
func (md *MEVDetector) DetectMEV(txs []*types.Transaction, receipts []*types.Receipt) *big.Int {
	totalMEV := big.NewInt(0)

	// Detect different types of MEV
	sandwichMEV := md.detectSandwichAttacks(txs, receipts)
	totalMEV.Add(totalMEV, sandwichMEV)

	arbitrageMEV := md.detectArbitrage(txs, receipts)
	totalMEV.Add(totalMEV, arbitrageMEV)

	liquidationMEV := md.detectLiquidations(txs, receipts)
	totalMEV.Add(totalMEV, liquidationMEV)

	frontrunMEV := md.detectFrontrunning(txs, receipts)
	totalMEV.Add(totalMEV, frontrunMEV)

	return totalMEV
}

// detectSandwichAttacks detects sandwich attacks in transactions
func (md *MEVDetector) detectSandwichAttacks(txs []*types.Transaction, receipts []*types.Receipt) *big.Int {
	totalMEV := big.NewInt(0)

	// Look for sandwich pattern: Bot TX → Victim TX → Bot TX
	for i := 1; i < len(txs)-1; i++ {
		prevTx := txs[i-1]
		currTx := txs[i]  // potential victim
		nextTx := txs[i+1]

		// Check if same address before and after (sandwich pattern)
		if prevTx.To() != nil && nextTx.To() != nil &&
		   *prevTx.To() == *nextTx.To() && // same contract
		   bytes.Equal(prevTx.Data()[:4], nextTx.Data()[:4]) && // same function
		   prevTx.From() == nextTx.From() && // same bot
		   prevTx.From() != currTx.From() { // different from victim

			// Check if these are swap transactions (DEX interactions)
			if md.isSwapTransaction(prevTx) && md.isSwapTransaction(currTx) && md.isSwapTransaction(nextTx) {
				// Calculate profit from sandwich
				profit := md.calculateSandwichProfit(prevTx, nextTx, receipts[i-1], receipts[i+1])
				if profit.Cmp(md.minProfitThreshold) > 0 {
					totalMEV.Add(totalMEV, profit)
				}
			}
		}
	}

	return totalMEV
}

// detectArbitrage detects arbitrage opportunities
func (md *MEVDetector) detectArbitrage(txs []*types.Transaction, receipts []*types.Receipt) *big.Int {
	totalMEV := big.NewInt(0)

	for i, tx := range txs {
		if i >= len(receipts) {
			continue
		}

		receipt := receipts[i]

		// Look for transactions that interact with multiple DEXs
		if md.isArbitrageTransaction(tx, receipt) {
			profit := md.calculateArbitrageProfit(receipt)
			if profit.Cmp(md.minProfitThreshold) > 0 {
				totalMEV.Add(totalMEV, profit)
			}
		}
	}

	return totalMEV
}

// detectLiquidations detects liquidation MEV
func (md *MEVDetector) detectLiquidations(txs []*types.Transaction, receipts []*types.Receipt) *big.Int {
	totalMEV := big.NewInt(0)

	for i, tx := range txs {
		if i >= len(receipts) {
			continue
		}

		if md.isLiquidationTransaction(tx) {
			profit := md.calculateLiquidationProfit(receipts[i])
			if profit.Cmp(md.minProfitThreshold) > 0 {
				totalMEV.Add(totalMEV, profit)
			}
		}
	}

	return totalMEV
}

// detectFrontrunning detects frontrunning attacks
func (md *MEVDetector) detectFrontrunning(txs []*types.Transaction, receipts []*types.Receipt) *big.Int {
	totalMEV := big.NewInt(0)

	// Look for transactions with much higher gas prices that execute same function before another tx
	for i := 0; i < len(txs)-1; i++ {
		tx1 := txs[i]
		tx2 := txs[i+1]

		// Check if tx1 frontran tx2
		if md.isFrontrunning(tx1, tx2) {
			profit := md.calculateFrontrunProfit(receipts[i])
			if profit.Cmp(md.minProfitThreshold) > 0 {
				totalMEV.Add(totalMEV, profit)
			}
		}
	}

	return totalMEV
}

// isSwapTransaction checks if a transaction is a token swap
func (md *MEVDetector) isSwapTransaction(tx *types.Transaction) bool {
	if tx.To() == nil || len(tx.Data()) < 4 {
		return false
	}

	// Common DEX function selectors
	swapSelectors := [][]byte{
		{0x38, 0xed, 0x17, 0x39}, // swapExactTokensForTokens (Uniswap V2)
		{0x8e, 0x3c, 0x5e, 0x16}, // swapExactTokensForETH (Uniswap V2)
		{0x7f, 0xf3, 0x6a, 0xb5}, // swapExactETHForTokens (Uniswap V2)
		{0x41, 0x4b, 0xf3, 0x89}, // exactInputSingle (Uniswap V3)
		{0xc0, 0x4b, 0x8d, 0x59}, // exactInput (Uniswap V3)
	}

	selector := tx.Data()[:4]
	for _, swapSelector := range swapSelectors {
		if bytes.Equal(selector, swapSelector) {
			return true
		}
	}

	return false
}

// isArbitrageTransaction checks if transaction performs arbitrage
func (md *MEVDetector) isArbitrageTransaction(tx *types.Transaction, receipt *types.Receipt) bool {
	// Arbitrage typically involves multiple swaps in same transaction
	swapEventCount := 0

	// Count swap events in logs
	for _, log := range receipt.Logs {
		if md.isSwapEvent(log) {
			swapEventCount++
		}
	}

	// Arbitrage usually has 2+ swaps
	return swapEventCount >= 2
}

// isLiquidationTransaction checks if transaction is a liquidation
func (md *MEVDetector) isLiquidationTransaction(tx *types.Transaction) bool {
	if tx.To() == nil || len(tx.Data()) < 4 {
		return false
	}

	// Common liquidation function selectors
	liquidationSelectors := [][]byte{
		{0x24, 0x96, 0x96, 0xf8}, // liquidateBorrow (Compound)
		{0x5c, 0x19, 0xa9, 0x5c}, // liquidationCall (Aave)
	}

	selector := tx.Data()[:4]
	for _, liqSelector := range liquidationSelectors {
		if bytes.Equal(selector, liqSelector) {
			return true
		}
	}

	return false
}

// isFrontrunning checks if tx1 frontran tx2
func (md *MEVDetector) isFrontrunning(tx1, tx2 *types.Transaction) bool {
	// Basic frontrunning detection:
	// 1. Same target contract
	// 2. Same function
	// 3. Much higher gas price
	// 4. Similar transaction value/parameters

	if tx1.To() == nil || tx2.To() == nil {
		return false
	}

	if *tx1.To() != *tx2.To() {
		return false
	}

	if len(tx1.Data()) < 4 || len(tx2.Data()) < 4 {
		return false
	}

	// Same function selector
	if !bytes.Equal(tx1.Data()[:4], tx2.Data()[:4]) {
		return false
	}

	// tx1 has significantly higher gas price (potential frontrun)
	gasPriceDiff := new(big.Int).Sub(tx1.GasPrice(), tx2.GasPrice())
	threshold := new(big.Int).Mul(tx2.GasPrice(), big.NewInt(20)) // 20x higher
	threshold.Div(threshold, big.NewInt(100))

	return gasPriceDiff.Cmp(threshold) > 0
}

// isSwapEvent checks if a log represents a swap event
func (md *MEVDetector) isSwapEvent(log *types.Log) bool {
	// Uniswap V2 Swap event topic
	swapTopic := common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822")

	if len(log.Topics) > 0 && log.Topics[0] == swapTopic {
		return true
	}

	return false
}

// Helper functions to calculate profits (simplified implementations)

func (md *MEVDetector) calculateSandwichProfit(frontrun, backrun *types.Transaction, frontrunReceipt, backrunReceipt *types.Receipt) *big.Int {
	// Simplified: estimate profit based on gas used and transaction values
	// In reality, this would analyze token transfers and price impacts

	if frontrun.Value().Cmp(backrun.Value()) <= 0 {
		return big.NewInt(0)
	}

	profit := new(big.Int).Sub(backrun.Value(), frontrun.Value())
	return profit
}

func (md *MEVDetector) calculateArbitrageProfit(receipt *types.Receipt) *big.Int {
	// Simplified: look at token transfer events to estimate profit
	// Real implementation would analyze all transfers and calculate net gain
	return big.NewInt(1e16) // Placeholder: 0.01 EQUA
}

func (md *MEVDetector) calculateLiquidationProfit(receipt *types.Receipt) *big.Int {
	// Simplified: estimate based on liquidation bonus/penalty
	return big.NewInt(5e16) // Placeholder: 0.05 EQUA
}

func (md *MEVDetector) calculateFrontrunProfit(receipt *types.Receipt) *big.Int {
	// Simplified: estimate profit from frontrunning
	return big.NewInt(2e16) // Placeholder: 0.02 EQUA
}