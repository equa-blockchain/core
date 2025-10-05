// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"bytes"
	"sort"
	"sync"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/log"
	"github.com/equa/go-equa/params"
)

// FairOrderer implements fair transaction ordering using First-Come-First-Served (FCFS)
type FairOrderer struct {
	config        *params.EquaConfig
	txTimestamps  map[common.Hash]time.Time // Transaction hash -> arrival timestamp
	txMutex       sync.RWMutex
	orderingStats OrderingStats
}

// OrderingStats tracks ordering statistics
type OrderingStats struct {
	TotalTransactions    uint64  `json:"totalTransactions"`
	OrderingViolations   uint64  `json:"orderingViolations"`
	AverageOrderingScore float64 `json:"averageOrderingScore"`
	LastOrderingScore    float64 `json:"lastOrderingScore"`
	FairOrderingRate     float64 `json:"fairOrderingRate"`
}

// NewFairOrderer creates a new fair orderer
func NewFairOrderer(config *params.EquaConfig) *FairOrderer {
	return &FairOrderer{
		config:       config,
		txTimestamps: make(map[common.Hash]time.Time),
		orderingStats: OrderingStats{
			AverageOrderingScore: 1.0,
			FairOrderingRate:     1.0,
		},
	}
}

// OrderTransactions orders transactions fairly using multi-dimensional fairness criteria
func (fo *FairOrderer) OrderTransactions(txs []*types.Transaction) []*types.Transaction {
	if len(txs) <= 1 {
		return txs
	}

	// Create a slice of transaction wrappers with enhanced metadata
	type txWrapper struct {
		tx            *types.Transaction
		timestamp     time.Time
		gasPrice      uint64
		priority      int
		hash          common.Hash
		fairnessScore float64
		mevRisk       float64
		nonce         uint64
		from          common.Address
	}

	wrapped := make([]txWrapper, len(txs))
	for i, tx := range txs {
		hash := tx.Hash()
		timestamp := fo.getTransactionTimestamp(tx)

		// Store timestamp for future reference
		fo.txMutex.Lock()
		fo.txTimestamps[hash] = timestamp
		fo.txMutex.Unlock()

		// Calculate multi-dimensional scores
		priority := fo.calculatePriority(tx)
		fairnessScore := fo.calculateFairnessScore(tx)
		mevRisk := fo.calculateMEVRisk(tx)

		// Get transaction sender
		from := common.Address{}
		if sender, err := types.Sender(types.LatestSigner(nil), tx); err == nil {
			from = sender
		}

		wrapped[i] = txWrapper{
			tx:            tx,
			timestamp:     timestamp,
			gasPrice:      tx.GasPrice().Uint64(),
			priority:      priority,
			hash:          hash,
			fairnessScore: fairnessScore,
			mevRisk:       mevRisk,
			nonce:         tx.Nonce(),
			from:          from,
		}
	}

	// Advanced multi-dimensional sorting with fairness optimization
	sort.Slice(wrapped, func(i, j int) bool {
		// 1. Penalize MEV-risky transactions (lower MEV risk first)
		if wrapped[i].mevRisk != wrapped[j].mevRisk {
			return wrapped[i].mevRisk < wrapped[j].mevRisk
		}

		// 2. Maintain nonce ordering for same sender (critical for validity)
		if wrapped[i].from == wrapped[j].from && wrapped[i].from != (common.Address{}) {
			return wrapped[i].nonce < wrapped[j].nonce
		}

		// 3. Fairness score (higher fairness first)
		if wrapped[i].fairnessScore != wrapped[j].fairnessScore {
			return wrapped[i].fairnessScore > wrapped[j].fairnessScore
		}

		// 4. Priority based on transaction type
		if wrapped[i].priority != wrapped[j].priority {
			return wrapped[i].priority > wrapped[j].priority
		}

		// 5. FCFS: timestamp ordering (earlier first)
		if !wrapped[i].timestamp.Equal(wrapped[j].timestamp) {
			return wrapped[i].timestamp.Before(wrapped[j].timestamp)
		}

		// 6. Gas price tiebreaker (higher first)
		return wrapped[i].gasPrice > wrapped[j].gasPrice
	})

	// Extract ordered transactions
	ordered := make([]*types.Transaction, len(txs))
	for i, w := range wrapped {
		ordered[i] = w.tx
	}

	// Validate and optimize ordering
	ordered = fo.validateAndOptimizeOrdering(ordered)

	// Update statistics
	fo.updateOrderingStats(ordered)

	log.Debug("📋 Transactions ordered (advanced)",
		"count", len(txs),
		"orderingScore", fo.GetOrderingScore(ordered),
		"fairOrdering", fo.ValidateOrdering(ordered),
		"violations", len(fo.DetectOrderingViolations(ordered)))

	return ordered
}

// getTransactionTimestamp gets the timestamp when transaction was received
func (fo *FairOrderer) getTransactionTimestamp(tx *types.Transaction) time.Time {
	hash := tx.Hash()

	// Check if we have a stored timestamp
	fo.txMutex.RLock()
	if timestamp, exists := fo.txTimestamps[hash]; exists {
		fo.txMutex.RUnlock()
		return timestamp
	}
	fo.txMutex.RUnlock()

	// Generate deterministic timestamp from transaction hash
	// This ensures consistent ordering across nodes
	timestamp := fo.generateDeterministicTimestamp(hash)

	// Store for future reference
	fo.txMutex.Lock()
	fo.txTimestamps[hash] = timestamp
	fo.txMutex.Unlock()

	return timestamp
}

// generateDeterministicTimestamp generates a deterministic timestamp from transaction hash
func (fo *FairOrderer) generateDeterministicTimestamp(hash common.Hash) time.Time {
	// Use first 8 bytes of hash to create timestamp
	timestamp := int64(0)
	for i := 0; i < 8 && i < len(hash); i++ {
		timestamp = (timestamp << 8) | int64(hash[i])
	}

	// Use it as offset from a base time (current time - some offset)
	baseTime := time.Now().Add(-time.Hour) // 1 hour ago
	return baseTime.Add(time.Duration(timestamp) * time.Nanosecond)
}

// SeparateByPriority separates transactions into priority tiers
func (fo *FairOrderer) SeparateByPriority(txs []*types.Transaction) (urgent, normal []*types.Transaction) {
	// High gas price threshold for urgent transactions
	urgentThreshold := uint64(1000000000000) // 1000 gwei

	for _, tx := range txs {
		if tx.GasPrice().Uint64() >= urgentThreshold {
			urgent = append(urgent, tx)
		} else {
			normal = append(normal, tx)
		}
	}

	return urgent, normal
}

// ValidateOrdering checks if transactions are properly ordered
func (fo *FairOrderer) ValidateOrdering(txs []*types.Transaction) bool {
	if len(txs) <= 1 {
		return true
	}

	// Check if transactions are in timestamp order
	for i := 1; i < len(txs); i++ {
		prevTime := fo.getTransactionTimestamp(txs[i-1])
		currTime := fo.getTransactionTimestamp(txs[i])

		// Allow some tolerance for network latency (100ms)
		tolerance := time.Millisecond * 100
		if currTime.Add(tolerance).Before(prevTime) {
			return false
		}
	}

	return true
}

// GetOrderingScore calculates a score for transaction ordering quality
func (fo *FairOrderer) GetOrderingScore(txs []*types.Transaction) float64 {
	if len(txs) <= 1 {
		return 1.0
	}

	violations := 0
	total := len(txs) - 1

	for i := 1; i < len(txs); i++ {
		prevTime := fo.getTransactionTimestamp(txs[i-1])
		currTime := fo.getTransactionTimestamp(txs[i])

		// Check for ordering violations
		if currTime.Before(prevTime) {
			violations++
		}

		// Check for gas price manipulation
		if fo.isGasPriceManipulation(txs[i-1], txs[i]) {
			violations++
		}
	}

	score := float64(total-violations) / float64(total)

	// Update statistics
	fo.orderingStats.LastOrderingScore = score
	fo.orderingStats.OrderingViolations += uint64(violations)

	return score
}

// Helper functions for real ordering implementation

// calculatePriority calculates transaction priority based on type and characteristics
func (fo *FairOrderer) calculatePriority(tx *types.Transaction) int {
	priority := 0

	// Base priority
	priority += 100

	// High gas price transactions get higher priority
	gasPrice := tx.GasPrice().Uint64()
	if gasPrice > 1000000000000 { // > 1000 gwei
		priority += 50
	} else if gasPrice > 100000000000 { // > 100 gwei
		priority += 25
	}

	// Contract creation transactions get higher priority
	if tx.To() == nil {
		priority += 30
	}

	// High value transactions get higher priority
	value := tx.Value().Uint64()
	if value > 1000000000000000000 { // > 1 ETH
		priority += 20
	}

	// Check for MEV-like patterns (lower priority)
	if fo.isMEVLikeTransaction(tx) {
		priority -= 50
	}

	// Check for spam transactions (lower priority)
	if fo.isSpamTransaction(tx) {
		priority -= 100
	}

	return priority
}

// isGasPriceManipulation detects gas price manipulation
func (fo *FairOrderer) isGasPriceManipulation(tx1, tx2 *types.Transaction) bool {
	// Check for suspicious gas price jumps
	gas1 := tx1.GasPrice().Uint64()
	gas2 := tx2.GasPrice().Uint64()

	// If gas price jumps by more than 10x, it might be manipulation
	if gas2 > gas1*10 {
		return true
	}

	return false
}

// isMEVLikeTransaction detects MEV-like transaction patterns
func (fo *FairOrderer) isMEVLikeTransaction(tx *types.Transaction) bool {
	// Check for common MEV function selectors
	if tx.To() == nil || len(tx.Data()) < 4 {
		return false
	}

	mevSelectors := [][]byte{
		{0x38, 0xed, 0x17, 0x39}, // swapExactTokensForTokens
		{0x7f, 0xf3, 0x6a, 0xb5}, // swapExactETHForTokens
		{0x8e, 0x3c, 0x5e, 0x16}, // swapExactTokensForETH
		{0x41, 0x4b, 0xf3, 0x89}, // exactInputSingle
		{0x24, 0x96, 0x96, 0xf8}, // liquidateBorrow
		{0x5c, 0x19, 0xa9, 0x5c}, // liquidationCall
	}

	selector := tx.Data()[:4]
	for _, mevSelector := range mevSelectors {
		if bytes.Equal(selector, mevSelector) {
			return true
		}
	}

	return false
}

// isSpamTransaction detects spam transactions
func (fo *FairOrderer) isSpamTransaction(tx *types.Transaction) bool {
	// Check for very low gas price (potential spam)
	gasPrice := tx.GasPrice().Uint64()
	if gasPrice < 1000000000 { // < 1 gwei
		return true
	}

	// Check for very small value (potential spam)
	value := tx.Value().Uint64()
	if value < 1000000000000000 { // < 0.001 ETH
		return true
	}

	// Check for empty data (potential spam)
	if len(tx.Data()) == 0 {
		return true
	}

	return false
}

// updateOrderingStats updates ordering statistics
func (fo *FairOrderer) updateOrderingStats(txs []*types.Transaction) {
	fo.orderingStats.TotalTransactions += uint64(len(txs))

	// Calculate average ordering score
	score := fo.GetOrderingScore(txs)
	fo.orderingStats.AverageOrderingScore =
		(fo.orderingStats.AverageOrderingScore + score) / 2

	// Calculate fair ordering rate
	if fo.orderingStats.TotalTransactions > 0 {
		fo.orderingStats.FairOrderingRate =
			float64(fo.orderingStats.TotalTransactions-fo.orderingStats.OrderingViolations) /
			float64(fo.orderingStats.TotalTransactions)
	}
}

// GetStats returns ordering statistics
func (fo *FairOrderer) GetStats() OrderingStats {
	return fo.orderingStats
}

// ResetStats resets ordering statistics
func (fo *FairOrderer) ResetStats() {
	fo.orderingStats = OrderingStats{
		AverageOrderingScore: 1.0,
		FairOrderingRate:     1.0,
	}
}

// DetectOrderingViolations detects specific ordering violations
func (fo *FairOrderer) DetectOrderingViolations(txs []*types.Transaction) []OrderingViolation {
	violations := make([]OrderingViolation, 0)

	for i := 1; i < len(txs); i++ {
		prevTx := txs[i-1]
		currTx := txs[i]

		prevTime := fo.getTransactionTimestamp(prevTx)
		currTime := fo.getTransactionTimestamp(currTx)

		// Check for timestamp violations
		if currTime.Before(prevTime) {
			violations = append(violations, OrderingViolation{
				Type:        "TIMESTAMP_VIOLATION",
				Description: "Transaction ordered before earlier transaction",
				Tx1:         prevTx.Hash(),
				Tx2:         currTx.Hash(),
				Severity:    8,
			})
		}

		// Check for gas price manipulation
		if fo.isGasPriceManipulation(prevTx, currTx) {
			violations = append(violations, OrderingViolation{
				Type:        "GAS_PRICE_MANIPULATION",
				Description: "Suspicious gas price jump detected",
				Tx1:         prevTx.Hash(),
				Tx2:         currTx.Hash(),
				Severity:    6,
			})
		}
	}

	return violations
}

// OrderingViolation represents an ordering violation
type OrderingViolation struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Tx1         common.Hash `json:"tx1"`
	Tx2         common.Hash `json:"tx2"`
	Severity    int         `json:"severity"` // 1-10 scale
}

// OptimizeOrdering optimizes transaction ordering for better fairness
func (fo *FairOrderer) OptimizeOrdering(txs []*types.Transaction) []*types.Transaction {
	// Apply additional fairness optimizations
	optimized := make([]*types.Transaction, len(txs))
	copy(optimized, txs)

	// Sort by fairness score
	sort.Slice(optimized, func(i, j int) bool {
		scoreI := fo.calculateFairnessScore(optimized[i])
		scoreJ := fo.calculateFairnessScore(optimized[j])
		return scoreI > scoreJ
	})

	return optimized
}

// calculateFairnessScore calculates a fairness score for a transaction
func (fo *FairOrderer) calculateFairnessScore(tx *types.Transaction) float64 {
	score := 1.0

	// Higher gas price = higher fairness (user paid more)
	gasPrice := tx.GasPrice().Uint64()
	if gasPrice > 1000000000000 { // > 1000 gwei
		score += 0.5
	} else if gasPrice > 100000000000 { // > 100 gwei
		score += 0.3
	} else if gasPrice > 20000000000 { // > 20 gwei
		score += 0.1
	}

	// Higher value = higher fairness (user has more at stake)
	value := tx.Value().Uint64()
	if value > 1000000000000000000 { // > 1 ETH
		score += 0.3
	} else if value > 100000000000000000 { // > 0.1 ETH
		score += 0.1
	}

	// Age bonus: older transactions get fairness boost (anti-censorship)
	age := fo.calculateTransactionAge(tx)
	if age > 60 { // > 1 minute
		score += 0.2
	} else if age > 30 { // > 30 seconds
		score += 0.1
	}

	// MEV transactions get lower fairness score
	if fo.isMEVLikeTransaction(tx) {
		score -= 0.7
	}

	// Spam transactions get lower fairness score
	if fo.isSpamTransaction(tx) {
		score -= 0.8
	}

	// Ensure score is non-negative
	if score < 0 {
		score = 0
	}

	return score
}

// calculateMEVRisk calculates MEV risk score for a transaction (0.0 = no risk, 1.0 = high risk)
func (fo *FairOrderer) calculateMEVRisk(tx *types.Transaction) float64 {
	risk := 0.0

	// Check for MEV-like patterns
	if fo.isMEVLikeTransaction(tx) {
		risk += 0.5
	}

	// Very high gas price indicates potential MEV
	gasPrice := tx.GasPrice().Uint64()
	if gasPrice > 1000000000000 { // > 1000 gwei
		risk += 0.4
	} else if gasPrice > 100000000000 { // > 100 gwei
		risk += 0.2
	}

	// Large transaction data (complex operations)
	if len(tx.Data()) > 10000 {
		risk += 0.1
	}

	// Check for flashloan patterns
	if fo.hasFlashloanPattern(tx) {
		risk += 0.3
	}

	// Cap risk at 1.0
	if risk > 1.0 {
		risk = 1.0
	}

	return risk
}

// calculateTransactionAge calculates how old a transaction is in seconds
func (fo *FairOrderer) calculateTransactionAge(tx *types.Transaction) uint64 {
	timestamp := fo.getTransactionTimestamp(tx)
	age := time.Since(timestamp).Seconds()
	if age < 0 {
		age = 0
	}
	return uint64(age)
}

// hasFlashloanPattern checks if transaction has flashloan characteristics
func (fo *FairOrderer) hasFlashloanPattern(tx *types.Transaction) bool {
	if len(tx.Data()) < 4 {
		return false
	}

	// Check for flashloan function selectors
	flashloanSelectors := [][]byte{
		{0x5c, 0xfa, 0x42, 0xb5}, // flashLoan
		{0xab, 0x9c, 0x4b, 0x5d}, // flashBorrow
	}

	selector := tx.Data()[:4]
	for _, flSelector := range flashloanSelectors {
		if bytes.Equal(selector, flSelector) {
			return true
		}
	}

	return false
}

// validateAndOptimizeOrdering validates and optimizes transaction ordering
func (fo *FairOrderer) validateAndOptimizeOrdering(txs []*types.Transaction) []*types.Transaction {
	if len(txs) <= 1 {
		return txs
	}

	// Create optimized ordering
	optimized := make([]*types.Transaction, len(txs))
	copy(optimized, txs)

	// Pass 1: Fix nonce ordering issues
	optimized = fo.fixNonceOrdering(optimized)

	// Pass 2: Separate potential sandwich attacks
	optimized = fo.separateSandwichPatterns(optimized)

	// Pass 3: Apply anti-MEV shuffling
	optimized = fo.applyAntiMEVShuffling(optimized)

	return optimized
}

// fixNonceOrdering ensures transactions from same sender are in nonce order
func (fo *FairOrderer) fixNonceOrdering(txs []*types.Transaction) []*types.Transaction {
	// Group transactions by sender
	senderTxs := make(map[common.Address][]*types.Transaction)

	for _, tx := range txs {
		sender, err := types.Sender(types.LatestSigner(nil), tx)
		if err != nil {
			continue
		}
		senderTxs[sender] = append(senderTxs[sender], tx)
	}

	// Sort each sender's transactions by nonce
	for sender := range senderTxs {
		sort.Slice(senderTxs[sender], func(i, j int) bool {
			return senderTxs[sender][i].Nonce() < senderTxs[sender][j].Nonce()
		})
	}

	// Rebuild transaction list maintaining nonce order
	result := make([]*types.Transaction, 0, len(txs))
	processed := make(map[common.Hash]bool)

	for _, tx := range txs {
		if processed[tx.Hash()] {
			continue
		}

		sender, err := types.Sender(types.LatestSigner(nil), tx)
		if err != nil {
			result = append(result, tx)
			processed[tx.Hash()] = true
			continue
		}

		// Add all transactions from this sender in nonce order
		for _, senderTx := range senderTxs[sender] {
			if !processed[senderTx.Hash()] {
				result = append(result, senderTx)
				processed[senderTx.Hash()] = true
			}
		}
	}

	return result
}

// separateSandwichPatterns detects and separates potential sandwich attack patterns
func (fo *FairOrderer) separateSandwichPatterns(txs []*types.Transaction) []*types.Transaction {
	if len(txs) < 3 {
		return txs
	}

	result := make([]*types.Transaction, 0, len(txs))
	i := 0

	for i < len(txs) {
		// Check for sandwich pattern at position i
		if i+2 < len(txs) && fo.isSandwichPattern(txs[i], txs[i+1], txs[i+2]) {
			// Shuffle the middle transaction away from potential sandwich
			result = append(result, txs[i+1]) // Move victim first
			result = append(result, txs[i])   // Then frontrun
			result = append(result, txs[i+2]) // Then backrun
			i += 3
		} else {
			result = append(result, txs[i])
			i++
		}
	}

	return result
}

// isSandwichPattern checks if three consecutive transactions form a sandwich pattern
func (fo *FairOrderer) isSandwichPattern(tx1, tx2, tx3 *types.Transaction) bool {
	// Get senders
	sender1, err1 := types.Sender(types.LatestSigner(nil), tx1)
	sender2, err2 := types.Sender(types.LatestSigner(nil), tx2)
	sender3, err3 := types.Sender(types.LatestSigner(nil), tx3)

	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}

	// Check if same sender for tx1 and tx3, different for tx2
	if sender1 != sender3 || sender1 == sender2 {
		return false
	}

	// Check if all are DEX interactions
	if !fo.isMEVLikeTransaction(tx1) || !fo.isMEVLikeTransaction(tx3) {
		return false
	}

	// Check for similar gas prices on tx1 and tx3 (coordinated attack)
	gas1 := tx1.GasPrice().Uint64()
	gas3 := tx3.GasPrice().Uint64()
	gasDiff := gas1
	if gas3 > gas1 {
		gasDiff = gas3 - gas1
	} else {
		gasDiff = gas1 - gas3
	}

	// If gas prices are very similar, likely coordinated
	return gasDiff < gas1/10 // Within 10%
}

// applyAntiMEVShuffling applies controlled randomization to prevent MEV
func (fo *FairOrderer) applyAntiMEVShuffling(txs []*types.Transaction) []*types.Transaction {
	if len(txs) <= 2 {
		return txs
	}

	// Group transactions by time windows (e.g., 100ms windows)
	const windowSize = 100 * time.Millisecond
	windows := fo.groupTransactionsByTimeWindow(txs, windowSize)

	// Shuffle within each window
	result := make([]*types.Transaction, 0, len(txs))
	for _, window := range windows {
		// Shuffle using deterministic randomness based on block data
		shuffled := fo.deterministicShuffle(window)
		result = append(result, shuffled...)
	}

	return result
}

// groupTransactionsByTimeWindow groups transactions into time windows
func (fo *FairOrderer) groupTransactionsByTimeWindow(txs []*types.Transaction, windowSize time.Duration) [][]*types.Transaction {
	if len(txs) == 0 {
		return nil
	}

	windows := make([][]*types.Transaction, 0)
	currentWindow := make([]*types.Transaction, 0)
	windowStart := fo.getTransactionTimestamp(txs[0])

	for _, tx := range txs {
		timestamp := fo.getTransactionTimestamp(tx)
		if timestamp.Sub(windowStart) > windowSize {
			// Start new window
			if len(currentWindow) > 0 {
				windows = append(windows, currentWindow)
			}
			currentWindow = make([]*types.Transaction, 0)
			windowStart = timestamp
		}
		currentWindow = append(currentWindow, tx)
	}

	// Add last window
	if len(currentWindow) > 0 {
		windows = append(windows, currentWindow)
	}

	return windows
}

// deterministicShuffle shuffles transactions deterministically
func (fo *FairOrderer) deterministicShuffle(txs []*types.Transaction) []*types.Transaction {
	if len(txs) <= 1 {
		return txs
	}

	// Create shuffled copy
	shuffled := make([]*types.Transaction, len(txs))
	copy(shuffled, txs)

	// Use transaction hashes to create deterministic randomness
	seed := uint64(0)
	for _, tx := range txs {
		hash := tx.Hash()
		for i := 0; i < 8; i++ {
			seed ^= uint64(hash[i]) << (i * 8)
		}
	}

	// Fisher-Yates shuffle with deterministic randomness
	for i := len(shuffled) - 1; i > 0; i-- {
		seed = (seed * 1103515245 + 12345) & 0x7fffffff // Linear congruential generator
		j := int(seed % uint64(i+1))
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled
}
