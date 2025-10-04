// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"sort"
	"time"

	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/params"
)

// FairOrderer implements fair transaction ordering using First-Come-First-Served (FCFS)
type FairOrderer struct {
	config *params.EquaConfig
}

// NewFairOrderer creates a new fair orderer
func NewFairOrderer(config *params.EquaConfig) *FairOrderer {
	return &FairOrderer{
		config: config,
	}
}

// OrderTransactions orders transactions fairly based on timestamp of arrival
func (fo *FairOrderer) OrderTransactions(txs []*types.Transaction) []*types.Transaction {
	if len(txs) <= 1 {
		return txs
	}

	// Create a slice of transaction wrappers with timestamps
	type txWrapper struct {
		tx        *types.Transaction
		timestamp time.Time
		gasPrice  uint64
	}

	wrapped := make([]txWrapper, len(txs))
	for i, tx := range txs {
		wrapped[i] = txWrapper{
			tx:        tx,
			timestamp: fo.getTransactionTimestamp(tx),
			gasPrice:  tx.GasPrice().Uint64(),
		}
	}

	// Sort by timestamp (FCFS), with gas price as tiebreaker
	sort.Slice(wrapped, func(i, j int) bool {
		// First, compare timestamps
		if !wrapped[i].timestamp.Equal(wrapped[j].timestamp) {
			return wrapped[i].timestamp.Before(wrapped[j].timestamp)
		}

		// If timestamps are equal (or very close), use gas price as tiebreaker
		return wrapped[i].gasPrice > wrapped[j].gasPrice
	})

	// Extract ordered transactions
	ordered := make([]*types.Transaction, len(txs))
	for i, w := range wrapped {
		ordered[i] = w.tx
	}

	return ordered
}

// getTransactionTimestamp gets the timestamp when transaction was received
func (fo *FairOrderer) getTransactionTimestamp(tx *types.Transaction) time.Time {
	// In a real implementation, this would be stored when the transaction
	// first arrives at the mempool. For now, we'll use a placeholder.

	// Try to extract timestamp from transaction hash (deterministic ordering)
	hash := tx.Hash()

	// Convert first 8 bytes of hash to timestamp-like value
	timestamp := int64(0)
	for i := 0; i < 8 && i < len(hash); i++ {
		timestamp = (timestamp << 8) | int64(hash[i])
	}

	// Use it as offset from a base time
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
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

		if currTime.Before(prevTime) {
			violations++
		}
	}

	return float64(total-violations) / float64(total)
}