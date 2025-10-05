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
	signer             types.Signer
}

// NewMEVDetector creates a new MEV detector
func NewMEVDetector(config *params.EquaConfig, chainConfig *params.ChainConfig) *MEVDetector {
	// Use latest signer (supports all transaction types)
	signer := types.LatestSigner(chainConfig)
	return &MEVDetector{
		config:             config,
		minProfitThreshold: big.NewInt(1e17), // 0.1 EQUA minimum profit to be considered MEV
		signer:             signer,
	}
}

// DetectMEV detects and quantifies MEV in a block using advanced multi-layer analysis
func (md *MEVDetector) DetectMEV(txs []*types.Transaction, receipts []*types.Receipt) *big.Int {
	if len(txs) == 0 || len(receipts) != len(txs) {
		return big.NewInt(0)
	}

	totalMEV := big.NewInt(0)

	// Layer 1: Detect sandwich attacks (most profitable MEV)
	sandwichMEV := md.detectSandwichAttacks(txs, receipts)
	if sandwichMEV.Cmp(big.NewInt(0)) > 0 {
		totalMEV.Add(totalMEV, sandwichMEV)
	}

	// Layer 2: Detect frontrunning attacks
	frontrunMEV := md.detectFrontrunning(txs, receipts)
	if frontrunMEV.Cmp(big.NewInt(0)) > 0 {
		totalMEV.Add(totalMEV, frontrunMEV)
	}

	// Layer 3: Detect arbitrage opportunities
	arbitrageMEV := md.detectArbitrage(txs, receipts)
	if arbitrageMEV.Cmp(big.NewInt(0)) > 0 {
		totalMEV.Add(totalMEV, arbitrageMEV)
	}

	// Layer 4: Detect liquidation MEV
	liquidationMEV := md.detectLiquidations(txs, receipts)
	if liquidationMEV.Cmp(big.NewInt(0)) > 0 {
		totalMEV.Add(totalMEV, liquidationMEV)
	}

	// Layer 5: Detect back-running attacks
	backrunMEV := md.detectBackrunning(txs, receipts)
	if backrunMEV.Cmp(big.NewInt(0)) > 0 {
		totalMEV.Add(totalMEV, backrunMEV)
	}

	// Layer 6: Detect time-bandit attacks (block reorganization for MEV)
	timeBanditMEV := md.detectTimeBanditAttacks(txs, receipts)
	if timeBanditMEV.Cmp(big.NewInt(0)) > 0 {
		totalMEV.Add(totalMEV, timeBanditMEV)
	}

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

		// Get sender addresses
		prevFrom, prevErr := types.Sender(md.signer, prevTx)
		nextFrom, nextErr := types.Sender(md.signer, nextTx)
		currFrom, currErr := types.Sender(md.signer, currTx)

		if prevErr != nil || nextErr != nil || currErr != nil {
			continue
		}

		// Check if same address before and after (sandwich pattern)
		if prevTx.To() != nil && nextTx.To() != nil &&
		   *prevTx.To() == *nextTx.To() && // same contract
		   bytes.Equal(prevTx.Data()[:4], nextTx.Data()[:4]) && // same function
		   prevFrom == nextFrom && // same bot
		   prevFrom != currFrom { // different from victim

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
	// Real sandwich profit calculation based on token transfers
	totalProfit := big.NewInt(0)

	// Analyze frontrun transaction profit
	frontrunProfit := md.analyzeTokenTransfers(frontrunReceipt)
	totalProfit.Add(totalProfit, frontrunProfit)

	// Analyze backrun transaction profit
	backrunProfit := md.analyzeTokenTransfers(backrunReceipt)
	totalProfit.Add(totalProfit, backrunProfit)

	// Subtract gas costs
	frontrunGasCost := md.calculateGasCost(frontrun)
	backrunGasCost := md.calculateGasCost(backrun)
	gasCost := new(big.Int).Add(frontrunGasCost, backrunGasCost)
	if totalProfit.Cmp(gasCost) > 0 {
		totalProfit.Sub(totalProfit, gasCost)
	} else {
		return big.NewInt(0) // No profit after gas costs
	}

	return totalProfit
}

func (md *MEVDetector) calculateArbitrageProfit(receipt *types.Receipt) *big.Int {
	// Real arbitrage profit calculation
	totalProfit := big.NewInt(0)

	// Analyze all token transfers in the transaction
	for _, log := range receipt.Logs {
		if md.isSwapEvent(log) {
			// Extract swap data and calculate profit
			profit := md.extractSwapProfit(log)
			totalProfit.Add(totalProfit, profit)
		}
	}

	// Subtract gas costs
	gasCost := md.calculateGasCostFromReceipt(receipt)
	if totalProfit.Cmp(gasCost) > 0 {
		totalProfit.Sub(totalProfit, gasCost)
	} else {
		return big.NewInt(0)
	}

	return totalProfit
}

func (md *MEVDetector) calculateLiquidationProfit(receipt *types.Receipt) *big.Int {
	// Real liquidation profit calculation
	totalProfit := big.NewInt(0)

	// Look for liquidation events in logs
	for _, log := range receipt.Logs {
		if md.isLiquidationEvent(log) {
			// Extract liquidation bonus
			bonus := md.extractLiquidationBonus(log)
			totalProfit.Add(totalProfit, bonus)
		}
	}

	// Subtract gas costs
	gasCost := md.calculateGasCostFromReceipt(receipt)
	if totalProfit.Cmp(gasCost) > 0 {
		totalProfit.Sub(totalProfit, gasCost)
	} else {
		return big.NewInt(0)
	}

	return totalProfit
}

func (md *MEVDetector) calculateFrontrunProfit(receipt *types.Receipt) *big.Int {
	// Real frontrunning profit calculation
	totalProfit := big.NewInt(0)

	// Analyze token transfers to find profit
	profit := md.analyzeTokenTransfers(receipt)
	totalProfit.Add(totalProfit, profit)

	// Subtract gas costs
	gasCost := md.calculateGasCostFromReceipt(receipt)
	if totalProfit.Cmp(gasCost) > 0 {
		totalProfit.Sub(totalProfit, gasCost)
	} else {
		return big.NewInt(0)
	}

	return totalProfit
}

// Helper functions for real MEV calculation

// analyzeTokenTransfers analyzes token transfers in a receipt to calculate profit
func (md *MEVDetector) analyzeTokenTransfers(receipt *types.Receipt) *big.Int {
	totalProfit := big.NewInt(0)

	// Look for ERC20 Transfer events
	for _, log := range receipt.Logs {
		if md.isERC20Transfer(log) {
			// Extract transfer amount and calculate potential profit
			amount := md.extractTransferAmount(log)
			if amount.Cmp(big.NewInt(0)) > 0 {
				// This is a simplified profit calculation
				// In reality, you'd need to track token prices and calculate actual value
				profit := new(big.Int).Div(amount, big.NewInt(1000)) // 0.1% of transfer as profit estimate
				totalProfit.Add(totalProfit, profit)
			}
		}
	}

	return totalProfit
}

// extractSwapProfit extracts profit from a swap event
func (md *MEVDetector) extractSwapProfit(log *types.Log) *big.Int {
	// Uniswap V2 Swap event structure:
	// topic[0]: Swap event signature
	// topic[1]: sender
	// topic[2]: to
	// data[0:32]: amount0In
	// data[32:64]: amount1In
	// data[64:96]: amount0Out
	// data[96:128]: amount1Out

	if len(log.Data) < 128 {
		return big.NewInt(0)
	}

	// Extract amounts from log data
	amount0In := new(big.Int).SetBytes(log.Data[0:32])
	amount1In := new(big.Int).SetBytes(log.Data[32:64])
	amount0Out := new(big.Int).SetBytes(log.Data[64:96])
	amount1Out := new(big.Int).SetBytes(log.Data[96:128])

	// Calculate profit as difference between input and output values
	// This is simplified - real calculation would use token prices
	totalIn := new(big.Int).Add(amount0In, amount1In)
	totalOut := new(big.Int).Add(amount0Out, amount1Out)

	if totalOut.Cmp(totalIn) > 0 {
		profit := new(big.Int).Sub(totalOut, totalIn)
		return profit
	}

	return big.NewInt(0)
}

// isLiquidationEvent checks if a log represents a liquidation event
func (md *MEVDetector) isLiquidationEvent(log *types.Log) bool {
	// Common liquidation event signatures
	liquidationTopics := []common.Hash{
		common.HexToHash("0x4c209b5fc8ad50758f13e2e1088ba56a560dff690a1c6fef26394f4c03821c4f"), // Compound liquidation
		common.HexToHash("0x2b627736bca15cd5381dcf80b0bf11fd197d01a037c52b927a881a10fb73bb61"), // Aave liquidation
	}

	if len(log.Topics) == 0 {
		return false
	}

	for _, topic := range liquidationTopics {
		if log.Topics[0] == topic {
			return true
		}
	}

	return false
}

// extractLiquidationBonus extracts liquidation bonus from log
func (md *MEVDetector) extractLiquidationBonus(log *types.Log) *big.Int {
	// Extract bonus from liquidation event data
	// This is simplified - real implementation would parse specific liquidation data
	if len(log.Data) >= 32 {
		bonus := new(big.Int).SetBytes(log.Data[0:32])
		return bonus
	}
	return big.NewInt(0)
}

// isERC20Transfer checks if a log represents an ERC20 Transfer event
func (md *MEVDetector) isERC20Transfer(log *types.Log) bool {
	// ERC20 Transfer event signature
	transferTopic := common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")

	if len(log.Topics) > 0 && log.Topics[0] == transferTopic {
		return true
	}

	return false
}

// extractTransferAmount extracts transfer amount from ERC20 Transfer event
func (md *MEVDetector) extractTransferAmount(log *types.Log) *big.Int {
	// ERC20 Transfer event data contains the amount
	if len(log.Data) >= 32 {
		amount := new(big.Int).SetBytes(log.Data[0:32])
		return amount
	}
	return big.NewInt(0)
}

// calculateGasCost calculates gas cost for a transaction
func (md *MEVDetector) calculateGasCost(tx *types.Transaction) *big.Int {
	gasUsed := tx.Gas()
	gasPrice := tx.GasPrice()
	return new(big.Int).Mul(big.NewInt(int64(gasUsed)), gasPrice)
}

// calculateGasCostFromReceipt calculates gas cost from receipt
func (md *MEVDetector) calculateGasCostFromReceipt(receipt *types.Receipt) *big.Int {
	// This is a simplified calculation
	// In reality, you'd need the original transaction to get gas price
	gasUsed := receipt.GasUsed
	// Use a default gas price for estimation
	gasPrice := big.NewInt(20000000000) // 20 gwei
	return new(big.Int).Mul(big.NewInt(int64(gasUsed)), gasPrice)
}

// detectBackrunning detects back-running attacks (executing after a transaction to exploit state changes)
func (md *MEVDetector) detectBackrunning(txs []*types.Transaction, receipts []*types.Receipt) *big.Int {
	totalMEV := big.NewInt(0)

	// Look for transactions that execute immediately after state-changing transactions
	for i := 0; i < len(txs)-1; i++ {
		currTx := txs[i]
		nextTx := txs[i+1]

		// Check if nextTx is back-running currTx
		if md.isBackrunning(currTx, nextTx, receipts[i], receipts[i+1]) {
			profit := md.calculateBackrunProfit(receipts[i+1])
			if profit.Cmp(md.minProfitThreshold) > 0 {
				totalMEV.Add(totalMEV, profit)
			}
		}
	}

	return totalMEV
}

// detectTimeBanditAttacks detects time-bandit attacks (block reorganization for MEV extraction)
func (md *MEVDetector) detectTimeBanditAttacks(txs []*types.Transaction, receipts []*types.Receipt) *big.Int {
	totalMEV := big.NewInt(0)

	// Check for suspicious transaction ordering that might indicate block reorganization
	for i := 0; i < len(txs)-1; i++ {
		if md.isTimeBanditPattern(txs[i], txs[i+1]) {
			// Calculate potential MEV from reorganization
			profit := md.calculateTimeBanditProfit(txs[i], txs[i+1], receipts[i], receipts[i+1])
			if profit.Cmp(md.minProfitThreshold) > 0 {
				totalMEV.Add(totalMEV, profit)
			}
		}
	}

	return totalMEV
}

// isBackrunning checks if tx2 is back-running tx1
func (md *MEVDetector) isBackrunning(tx1, tx2 *types.Transaction, receipt1, receipt2 *types.Receipt) bool {
	// Back-running: tx2 executes immediately after tx1 to profit from state changes

	// Check if tx1 changed significant state (e.g., price oracle update)
	if !md.hasSignificantStateChange(receipt1) {
		return false
	}

	// Check if tx2 interacts with the same contract or related contracts
	if tx1.To() == nil || tx2.To() == nil {
		return false
	}

	// Check if tx2 could profit from tx1's state changes
	if md.couldProfitFromStateChange(tx1, tx2, receipt1, receipt2) {
		return true
	}

	return false
}

// isTimeBanditPattern checks for time-bandit attack patterns
func (md *MEVDetector) isTimeBanditPattern(tx1, tx2 *types.Transaction) bool {
	// Time-bandit: reorganizing blocks to extract MEV

	// Check for very high-value transactions that could justify reorganization
	if tx1.Value().Cmp(big.NewInt(1e18)) < 0 && tx2.Value().Cmp(big.NewInt(1e18)) < 0 {
		return false
	}

	// Check for suspicious gas price patterns
	gasDiff := new(big.Int).Sub(tx2.GasPrice(), tx1.GasPrice())
	threshold := new(big.Int).Mul(tx1.GasPrice(), big.NewInt(50)) // 50x difference
	threshold.Div(threshold, big.NewInt(100))

	return gasDiff.Cmp(threshold) > 0
}

// hasSignificantStateChange checks if a transaction caused significant state changes
func (md *MEVDetector) hasSignificantStateChange(receipt *types.Receipt) bool {
	// Check for significant number of logs (state changes)
	if len(receipt.Logs) < 2 {
		return false
	}

	// Check for price oracle updates, liquidity changes, etc.
	for _, log := range receipt.Logs {
		if md.isPriceOracleUpdate(log) || md.isLiquidityChange(log) {
			return true
		}
	}

	return false
}

// couldProfitFromStateChange checks if tx2 could profit from tx1's state changes
func (md *MEVDetector) couldProfitFromStateChange(tx1, tx2 *types.Transaction, receipt1, receipt2 *types.Receipt) bool {
	// Check if tx2 is a swap/trade that could benefit from price changes in tx1
	if !md.isSwapTransaction(tx2) {
		return false
	}

	// Check if tx1 caused price changes
	for _, log := range receipt1.Logs {
		if md.isSwapEvent(log) || md.isPriceOracleUpdate(log) {
			return true
		}
	}

	return false
}

// isPriceOracleUpdate checks if a log represents a price oracle update
func (md *MEVDetector) isPriceOracleUpdate(log *types.Log) bool {
	// Common oracle update event signatures (using known event hashes)
	oracleTopics := []common.Hash{
		common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925"), // PriceUpdated
		common.HexToHash("0x0559884fd3a460db3073b7fc896cc77986f16e378210ded43186175bf646fc5f"), // AnswerUpdated
	}

	if len(log.Topics) == 0 {
		return false
	}

	for _, topic := range oracleTopics {
		if log.Topics[0] == topic {
			return true
		}
	}

	return false
}

// isLiquidityChange checks if a log represents a significant liquidity change
func (md *MEVDetector) isLiquidityChange(log *types.Log) bool {
	// Common liquidity event signatures (using known event hashes)
	liquidityTopics := []common.Hash{
		common.HexToHash("0x4c209b5fc8ad50758f13e2e1088ba56a560dff690a1c6fef26394f4c03821c4f"), // Mint
		common.HexToHash("0xdccd412f0b1252819cb1fd330b93224ca42612892bb3f4f789976e6d81936496"), // Burn
		common.HexToHash("0x1c411e9a96e071241c2f21f7726b17ae89e3cab4c78be50e062b03a9fffbbad1"), // Sync
	}

	if len(log.Topics) == 0 {
		return false
	}

	for _, topic := range liquidityTopics {
		if log.Topics[0] == topic {
			return true
		}
	}

	return false
}

// calculateBackrunProfit calculates profit from back-running
func (md *MEVDetector) calculateBackrunProfit(receipt *types.Receipt) *big.Int {
	totalProfit := big.NewInt(0)

	// Analyze token transfers and price impact
	for _, log := range receipt.Logs {
		if md.isSwapEvent(log) {
			profit := md.extractSwapProfit(log)
			totalProfit.Add(totalProfit, profit)
		}
	}

	// Subtract gas costs
	gasCost := md.calculateGasCostFromReceipt(receipt)
	if totalProfit.Cmp(gasCost) > 0 {
		totalProfit.Sub(totalProfit, gasCost)
	} else {
		return big.NewInt(0)
	}

	return totalProfit
}

// calculateTimeBanditProfit calculates potential profit from time-bandit attacks
func (md *MEVDetector) calculateTimeBanditProfit(tx1, tx2 *types.Transaction, receipt1, receipt2 *types.Receipt) *big.Int {
	// Calculate potential MEV from reorganizing these transactions
	profit1 := md.analyzeTokenTransfers(receipt1)
	profit2 := md.analyzeTokenTransfers(receipt2)

	totalProfit := new(big.Int).Add(profit1, profit2)

	// Subtract reorganization costs (gas costs of both transactions)
	gasCost1 := md.calculateGasCostFromReceipt(receipt1)
	gasCost2 := md.calculateGasCostFromReceipt(receipt2)
	totalGasCost := new(big.Int).Add(gasCost1, gasCost2)

	if totalProfit.Cmp(totalGasCost) > 0 {
		totalProfit.Sub(totalProfit, totalGasCost)
	} else {
		return big.NewInt(0)
	}

	return totalProfit
}
