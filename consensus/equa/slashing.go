// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"bytes"
	"crypto/sha256"
	"math/big"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/log"
	"github.com/equa/go-equa/params"
)

// Slasher detects malicious behavior and applies penalties
type Slasher struct {
	config           *params.EquaConfig
	signer           types.Signer
	violationHistory map[common.Address][]ViolationRecord
	slashingEvents   []SlashingEvent
}

// ViolationRecord tracks a validator's violation
type ViolationRecord struct {
	Type        string    `json:"type"`
	BlockNumber uint64    `json:"blockNumber"`
	Timestamp   time.Time `json:"timestamp"`
	Severity    uint8     `json:"severity"` // 1-10 scale
	Evidence    []byte    `json:"evidence"`
}

// SlashingEvent represents a slashing event
type SlashingEvent struct {
	Validator     common.Address `json:"validator"`
	Amount        *big.Int       `json:"amount"`
	Reason        string         `json:"reason"`
	BlockNumber   uint64         `json:"blockNumber"`
	Timestamp     time.Time      `json:"timestamp"`
	Evidence      []byte         `json:"evidence"`
}

// NewSlasher creates a new slasher
func NewSlasher(config *params.EquaConfig, chainConfig *params.ChainConfig) *Slasher {
	return &Slasher{
		config:           config,
		signer:           types.LatestSigner(chainConfig),
		violationHistory: make(map[common.Address][]ViolationRecord),
		slashingEvents:   make([]SlashingEvent, 0),
	}
}

// DetectMEVExtraction detects if a validator extracted MEV using multi-layer analysis
func (s *Slasher) DetectMEVExtraction(validator common.Address, txs []*types.Transaction, receipts []*types.Receipt) bool {
	if len(txs) == 0 || len(receipts) != len(txs) {
		return false
	}

	mevDetected := false
	evidence := make([]byte, 0)
	violationScore := 0 // Cumulative severity score

	// Layer 1: Check for validator's own MEV transactions (direct involvement)
	validatorTxs := s.getValidatorTransactions(validator, txs)
	for i, tx := range validatorTxs {
		if s.isMEVTransaction(tx) {
			mevDetected = true
			violationScore += 3
			evidence = append(evidence, s.generateMEVEvidence(tx, receipts[i])...)
		}
	}

	// Layer 2: Check for sandwich attacks by validator (high severity)
	if s.detectSandwichAttack(validator, txs, receipts) {
		mevDetected = true
		violationScore += 5
		evidence = append(evidence, s.generateSandwichEvidence(validator, txs)...)
	}

	// Layer 3: Check for frontrunning by validator (medium-high severity)
	if s.detectFrontrunning(validator, txs, receipts) {
		mevDetected = true
		violationScore += 4
		evidence = append(evidence, s.generateFrontrunEvidence(validator, txs)...)
	}

	// Layer 4: Check for arbitrage by validator (medium severity)
	if s.detectArbitrage(validator, txs, receipts) {
		mevDetected = true
		violationScore += 2
		evidence = append(evidence, s.generateArbitrageEvidence(validator, txs)...)
	}

	// Layer 5: Check for liquidation MEV by validator (medium severity)
	if s.detectLiquidationMEV(validator, txs, receipts) {
		mevDetected = true
		violationScore += 3
		evidence = append(evidence, s.generateLiquidationEvidence(validator, txs)...)
	}

	// Layer 6: Check for back-running by validator
	if s.detectBackrunning(validator, txs, receipts) {
		mevDetected = true
		violationScore += 3
		evidence = append(evidence, s.generateBackrunEvidence(validator, txs)...)
	}

	// Layer 7: Check for transaction censorship (very high severity)
	if s.detectTransactionCensorship(validator, txs) {
		mevDetected = true
		violationScore += 6
		evidence = append(evidence, s.generateCensorshipEvidence(validator, txs)...)
	}

	// Layer 8: Check for uncle block MEV extraction
	if s.detectUncleBlockMEV(validator, txs, receipts) {
		mevDetected = true
		violationScore += 4
		evidence = append(evidence, s.generateUncleBlockEvidence(validator, txs)...)
	}

	// Record violation if MEV detected (severity based on cumulative score)
	if mevDetected {
		severity := s.calculateViolationSeverity(violationScore)
		s.recordViolation(validator, "MEV_EXTRACTION", evidence, severity)

		log.Warn("🚨 MEV extraction detected",
			"validator", validator.Hex()[:10]+"...",
			"severity", severity,
			"score", violationScore,
			"evidenceLen", len(evidence))
	}

	return mevDetected
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

	// Check for suspicious gaps in gas prices (potential MEV)
	// Look for transactions with very different gas prices that could indicate front-running
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
		from, err := types.Sender(s.signer, tx)
		if err != nil {
			continue
		}
		if from == validator {
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
	case "MEV_EXTRACTION":
		percentage = s.config.SlashingPercentage
	case "TRANSACTION_REORDERING":
		percentage = 10
	case "TRANSACTION_CENSORSHIP":
		percentage = 20
	case "VALIDATOR_COLLUSION":
		percentage = 100 // Total slash for collusion
	case "DOUBLE_SIGNING":
		percentage = 100 // Total slash for double signing
	case "INVALID_BLOCK":
		percentage = 50
	default:
		percentage = 5 // Default minor slash
	}

	slashAmount := new(big.Int).Mul(stake, big.NewInt(int64(percentage)))
	slashAmount.Div(slashAmount, big.NewInt(100))

	return slashAmount
}

// Helper functions for real slashing implementation

// getValidatorTransactions returns transactions from a specific validator
func (s *Slasher) getValidatorTransactions(validator common.Address, txs []*types.Transaction) []*types.Transaction {
	validatorTxs := make([]*types.Transaction, 0)

	for _, tx := range txs {
		from, err := types.Sender(s.signer, tx)
		if err != nil {
			continue
		}
		if from == validator {
			validatorTxs = append(validatorTxs, tx)
		}
	}

	return validatorTxs
}

// detectSandwichAttack detects sandwich attacks by validator
func (s *Slasher) detectSandwichAttack(validator common.Address, txs []*types.Transaction, receipts []*types.Receipt) bool {
	// Look for pattern: Validator TX → Victim TX → Validator TX
	for i := 1; i < len(txs)-1; i++ {
		prevTx := txs[i-1]
		currTx := txs[i]
		nextTx := txs[i+1]

		prevFrom, prevErr := types.Sender(s.signer, prevTx)
		nextFrom, nextErr := types.Sender(s.signer, nextTx)
		currFrom, currErr := types.Sender(s.signer, currTx)

		if prevErr != nil || nextErr != nil || currErr != nil {
			continue
		}

		// Check if validator sandwiched a victim
		if prevFrom == validator && nextFrom == validator && currFrom != validator {
		// Check if all are DEX interactions
		if s.isDEXInteractionSlasher(prevTx) && s.isDEXInteractionSlasher(currTx) && s.isDEXInteractionSlasher(nextTx) {
				// Check if same token pair
				if s.sameTokenPair(prevTx, nextTx) {
					return true
				}
			}
		}
	}
	return false
}

// detectFrontrunning detects frontrunning by validator
func (s *Slasher) detectFrontrunning(validator common.Address, txs []*types.Transaction, receipts []*types.Receipt) bool {
	for i := 0; i < len(txs)-1; i++ {
		tx1 := txs[i]
		tx2 := txs[i+1]

		from1, err1 := types.Sender(s.signer, tx1)
		from2, err2 := types.Sender(s.signer, tx2)

		if err1 != nil || err2 != nil {
			continue
		}

		// Check if validator frontran another transaction
		if from1 == validator && from2 != validator {
			// Same contract and function
			if tx1.To() != nil && tx2.To() != nil &&
			   *tx1.To() == *tx2.To() &&
			   len(tx1.Data()) >= 4 && len(tx2.Data()) >= 4 &&
			   bytes.Equal(tx1.Data()[:4], tx2.Data()[:4]) {

				// Much higher gas price
				gasDiff := new(big.Int).Sub(tx1.GasPrice(), tx2.GasPrice())
				threshold := new(big.Int).Mul(tx2.GasPrice(), big.NewInt(20))
				threshold.Div(threshold, big.NewInt(100))

				if gasDiff.Cmp(threshold) > 0 {
					return true
				}
			}
		}
	}
	return false
}

// detectArbitrage detects arbitrage by validator
func (s *Slasher) detectArbitrage(validator common.Address, txs []*types.Transaction, receipts []*types.Receipt) bool {
	for i, tx := range txs {
		from, err := types.Sender(s.signer, tx)
		if err != nil {
			continue
		}
		if from == validator && i < len(receipts) {
			// Check for multiple DEX interactions in same transaction
			swapCount := 0
			for _, log := range receipts[i].Logs {
				if s.isSwapEvent(log) {
					swapCount++
				}
			}
			// Arbitrage typically involves 2+ swaps
			if swapCount >= 2 {
				return true
			}
		}
	}
	return false
}

// detectLiquidationMEV detects liquidation MEV by validator
func (s *Slasher) detectLiquidationMEV(validator common.Address, txs []*types.Transaction, receipts []*types.Receipt) bool {
	for i, tx := range txs {
		from, err := types.Sender(s.signer, tx)
		if err != nil {
			continue
		}
		if from == validator && i < len(receipts) {
			// Check for liquidation events
			for _, log := range receipts[i].Logs {
				if s.isLiquidationEvent(log) {
					return true
				}
			}
		}
	}
	return false
}

// generateMEVEvidence generates cryptographic evidence of MEV
func (s *Slasher) generateMEVEvidence(tx *types.Transaction, receipt *types.Receipt) []byte {
	// Create evidence hash
	data := make([]byte, 0)
	data = append(data, tx.Hash().Bytes()...)
	data = append(data, receipt.TxHash.Bytes()...)

	// Add gas price as evidence
	gasPriceBytes := make([]byte, 32)
	tx.GasPrice().FillBytes(gasPriceBytes)
	data = append(data, gasPriceBytes...)

	// Add value as evidence
	valueBytes := make([]byte, 32)
	tx.Value().FillBytes(valueBytes)
	data = append(data, valueBytes...)

	// Hash the evidence
	hash := sha256.Sum256(data)
	return hash[:]
}

// generateSandwichEvidence generates evidence of sandwich attack
func (s *Slasher) generateSandwichEvidence(validator common.Address, txs []*types.Transaction) []byte {
	// Find sandwich pattern and create evidence
	for i := 1; i < len(txs)-1; i++ {
		prevTx := txs[i-1]
		currTx := txs[i]
		nextTx := txs[i+1]

		prevFrom, _ := types.Sender(s.signer, prevTx)
		nextFrom, _ := types.Sender(s.signer, nextTx)
		currFrom, _ := types.Sender(s.signer, currTx)

		if prevFrom == validator && nextFrom == validator && currFrom != validator {
			// Create evidence hash
			data := make([]byte, 0)
			data = append(data, prevTx.Hash().Bytes()...)
			data = append(data, currTx.Hash().Bytes()...)
			data = append(data, nextTx.Hash().Bytes()...)
			data = append(data, validator.Bytes()...)

			hash := sha256.Sum256(data)
			return hash[:]
		}
	}
	return []byte{}
}

// generateFrontrunEvidence generates evidence of frontrunning
func (s *Slasher) generateFrontrunEvidence(validator common.Address, txs []*types.Transaction) []byte {
	// Find frontrunning pattern and create evidence
	for i := 0; i < len(txs)-1; i++ {
		tx1 := txs[i]
		tx2 := txs[i+1]

		from1, _ := types.Sender(s.signer, tx1)
		from2, _ := types.Sender(s.signer, tx2)

		if from1 == validator && from2 != validator {
			// Create evidence hash
			data := make([]byte, 0)
			data = append(data, tx1.Hash().Bytes()...)
			data = append(data, tx2.Hash().Bytes()...)
			data = append(data, validator.Bytes()...)

			hash := sha256.Sum256(data)
			return hash[:]
		}
	}
	return []byte{}
}

// generateArbitrageEvidence generates evidence of arbitrage
func (s *Slasher) generateArbitrageEvidence(validator common.Address, txs []*types.Transaction) []byte {
	// Find arbitrage pattern and create evidence
	for _, tx := range txs {
		from, _ := types.Sender(s.signer, tx)
		if from == validator {
			// Create evidence hash
			data := make([]byte, 0)
			data = append(data, tx.Hash().Bytes()...)
			data = append(data, validator.Bytes()...)

			hash := sha256.Sum256(data)
			return hash[:]
		}
	}
	return []byte{}
}

// generateLiquidationEvidence generates evidence of liquidation MEV
func (s *Slasher) generateLiquidationEvidence(validator common.Address, txs []*types.Transaction) []byte {
	// Find liquidation pattern and create evidence
	for _, tx := range txs {
		from, _ := types.Sender(s.signer, tx)
		if from == validator {
			// Create evidence hash
			data := make([]byte, 0)
			data = append(data, tx.Hash().Bytes()...)
			data = append(data, validator.Bytes()...)

			hash := sha256.Sum256(data)
			return hash[:]
		}
	}
	return []byte{}
}

// recordViolation records a violation for a validator
func (s *Slasher) recordViolation(validator common.Address, violationType string, evidence []byte, severity uint8) {
	violation := ViolationRecord{
		Type:        violationType,
		BlockNumber: 0, // Will be set by caller
		Timestamp:   time.Now(),
		Severity:    severity,
		Evidence:    evidence,
	}

	s.violationHistory[validator] = append(s.violationHistory[validator], violation)
}

// isDEXInteractionSlasher checks if transaction interacts with a DEX (for slasher)
func (s *Slasher) isDEXInteractionSlasher(tx *types.Transaction) bool {
	if tx.To() == nil || len(tx.Data()) < 4 {
		return false
	}

	// Common DEX function selectors
	dexSelectors := [][]byte{
		{0x38, 0xed, 0x17, 0x39}, // swapExactTokensForTokens
		{0x7f, 0xf3, 0x6a, 0xb5}, // swapExactETHForTokens
		{0x8e, 0x3c, 0x5e, 0x16}, // swapExactTokensForETH
		{0x41, 0x4b, 0xf3, 0x89}, // exactInputSingle
	}

	selector := tx.Data()[:4]
	for _, dexSelector := range dexSelectors {
		if bytes.Equal(selector, dexSelector) {
			return true
		}
	}
	return false
}

// sameTokenPair checks if two transactions involve the same token pair
func (s *Slasher) sameTokenPair(tx1, tx2 *types.Transaction) bool {
	// Simplified check - in reality would parse function parameters
	return tx1.To() != nil && tx2.To() != nil && *tx1.To() == *tx2.To()
}

// isSwapEvent checks if a log represents a swap event
func (s *Slasher) isSwapEvent(log *types.Log) bool {
	swapTopic := common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822")
	return len(log.Topics) > 0 && log.Topics[0] == swapTopic
}

// isLiquidationEvent checks if a log represents a liquidation event
func (s *Slasher) isLiquidationEvent(log *types.Log) bool {
	liquidationTopics := []common.Hash{
		common.HexToHash("0x4c209b5fc8ad50758f13e2e1088ba56a560dff690a1c6fef26394f4c03821c4f"), // Compound
		common.HexToHash("0x2b627736bca15cd5381dcf80b0bf11fd197d01a037c52b927a881a10fb73bb61"), // Aave
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

// minInt returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// detectBackrunning detects back-running attacks by validator
func (s *Slasher) detectBackrunning(validator common.Address, txs []*types.Transaction, receipts []*types.Receipt) bool {
	for i := 0; i < len(txs)-1; i++ {
		currTx := txs[i]
		nextTx := txs[i+1]

		nextFrom, err := types.Sender(s.signer, nextTx)
		if err != nil || nextFrom != validator {
			continue
		}

		// Check if validator's transaction is back-running
		if s.isBackrunningPattern(currTx, nextTx, receipts[i], receipts[i+1]) {
			return true
		}
	}
	return false
}

// isBackrunningPattern checks if tx2 is back-running tx1
func (s *Slasher) isBackrunningPattern(tx1, tx2 *types.Transaction, receipt1, receipt2 *types.Receipt) bool {
	// Check if tx1 caused significant state changes
	if len(receipt1.Logs) < 2 {
		return false
	}

	// Check if tx2 could profit from tx1's state changes
	// (e.g., price oracle update followed by swap)
	hasStateChange := false
	for _, log := range receipt1.Logs {
		if s.isPriceChangeEvent(log) {
			hasStateChange = true
			break
		}
	}

	return hasStateChange && s.isDEXInteractionSlasher(tx2)
}

// isPriceChangeEvent checks if a log represents a price change
func (s *Slasher) isPriceChangeEvent(log *types.Log) bool {
	// Check for common price change events
	priceTopics := []common.Hash{
		common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"), // Swap
		common.HexToHash("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925"), // PriceUpdated
		common.HexToHash("0x1c411e9a96e071241c2f21f7726b17ae89e3cab4c78be50e062b03a9fffbbad1"), // Sync
	}

	if len(log.Topics) == 0 {
		return false
	}

	for _, topic := range priceTopics {
		if log.Topics[0] == topic {
			return true
		}
	}
	return false
}

// detectTransactionCensorship detects if validator is censoring transactions
func (s *Slasher) detectTransactionCensorship(validator common.Address, txs []*types.Transaction) bool {
	if len(txs) < 5 {
		// Too few transactions to determine censorship
		return false
	}

	// Check for suspicious patterns:
	// 1. Very few transactions despite high gas prices
	// 2. Missing high-fee transactions
	// 3. Validator's own transactions dominate

	validatorTxCount := 0
	totalValue := big.NewInt(0)
	highGasCount := 0

	for _, tx := range txs {
		from, err := types.Sender(s.signer, tx)
		if err != nil {
			continue
		}

		if from == validator {
			validatorTxCount++
		}

		totalValue.Add(totalValue, tx.Value())

		if tx.GasPrice().Cmp(big.NewInt(100000000000)) > 0 { // > 100 gwei
			highGasCount++
		}
	}

	// Suspicious if validator's transactions > 30% of block
	if float64(validatorTxCount)/float64(len(txs)) > 0.3 {
		return true
	}

	// Suspicious if very few high-gas transactions (potential censorship)
	if highGasCount < len(txs)/10 {
		return true
	}

	return false
}

// detectUncleBlockMEV detects MEV extraction via uncle blocks
func (s *Slasher) detectUncleBlockMEV(validator common.Address, txs []*types.Transaction, receipts []*types.Receipt) bool {
	// Check for transactions that appear to be extracting MEV
	// that would have been in uncle blocks

	// This is a simplified check - real implementation would
	// need access to uncle block data
	for i, tx := range txs {
		from, err := types.Sender(s.signer, tx)
		if err != nil || from != validator {
			continue
		}

		// Check for high-value, high-gas transactions (potential uncle MEV)
		if tx.Value().Cmp(big.NewInt(1000000000000000000)) > 0 && // > 1 ETH
			tx.GasPrice().Cmp(big.NewInt(100000000000)) > 0 { // > 100 gwei

			// Check if transaction has suspicious timing
			if i < 3 || i > len(txs)-3 {
				// Suspicious: positioned at block edges (potential uncle reorg)
				return true
			}
		}
	}

	return false
}

// calculateViolationSeverity calculates severity from cumulative score
func (s *Slasher) calculateViolationSeverity(score int) uint8 {
	// Map cumulative score to severity (1-10)
	if score >= 20 {
		return 10 // Critical
	} else if score >= 15 {
		return 9 // Very high
	} else if score >= 10 {
		return 8 // High
	} else if score >= 7 {
		return 7 // Medium-high
	} else if score >= 5 {
		return 6 // Medium
	} else if score >= 3 {
		return 5 // Medium-low
	} else if score >= 2 {
		return 4 // Low
	} else {
		return 3 // Very low
	}
}

// generateBackrunEvidence generates evidence of back-running
func (s *Slasher) generateBackrunEvidence(validator common.Address, txs []*types.Transaction) []byte {
	data := make([]byte, 0)
	data = append(data, validator.Bytes()...)

	// Add transaction hashes that show back-running pattern
	for i := 0; i < len(txs)-1; i++ {
		from, err := types.Sender(s.signer, txs[i+1])
		if err != nil || from != validator {
			continue
		}

		// Found validator's transaction that might be back-running
		data = append(data, txs[i].Hash().Bytes()...)
		data = append(data, txs[i+1].Hash().Bytes()...)
	}

	hash := sha256.Sum256(data)
	return hash[:]
}

// generateCensorshipEvidence generates evidence of censorship
func (s *Slasher) generateCensorshipEvidence(validator common.Address, txs []*types.Transaction) []byte {
	data := make([]byte, 0)
	data = append(data, validator.Bytes()...)

	// Add block statistics as evidence
	validatorTxCount := 0
	for _, tx := range txs {
		from, err := types.Sender(s.signer, tx)
		if err != nil {
			continue
		}
		if from == validator {
			validatorTxCount++
			data = append(data, tx.Hash().Bytes()...)
		}
	}

	hash := sha256.Sum256(data)
	return hash[:]
}

// generateUncleBlockEvidence generates evidence of uncle block MEV
func (s *Slasher) generateUncleBlockEvidence(validator common.Address, txs []*types.Transaction) []byte {
	data := make([]byte, 0)
	data = append(data, validator.Bytes()...)

	// Add suspicious transactions as evidence
	for _, tx := range txs {
		from, err := types.Sender(s.signer, tx)
		if err != nil || from != validator {
			continue
		}

		// High-value, high-gas transactions
		if tx.Value().Cmp(big.NewInt(1000000000000000000)) > 0 &&
			tx.GasPrice().Cmp(big.NewInt(100000000000)) > 0 {
			data = append(data, tx.Hash().Bytes()...)
		}
	}

	hash := sha256.Sum256(data)
	return hash[:]
}
