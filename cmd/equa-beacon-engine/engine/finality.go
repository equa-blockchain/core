// Copyright 2024 The go-equa Authors
// EQUA Beacon Engine - Fast Finality with Threshold Signatures

package engine

import (
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/log"
)

var (
	ErrInsufficientAttestations = errors.New("insufficient attestations for finality")
	ErrInvalidCheckpoint = errors.New("invalid finality checkpoint")
	ErrAlreadyFinalized = errors.New("block already finalized")
)

// FinalityEngine manages fast finality using threshold signatures
type FinalityEngine struct {
	mu sync.RWMutex

	config *Config
	state  *BeaconState

	// Attestation pool
	attestationPool *AttestationPool

	// Checkpoints
	checkpoints map[uint64]*FinalityCheckpoint
	finalizedCheckpoints []*FinalityCheckpoint

	// Latest finalized
	latestFinalized     *FinalityCheckpoint
	latestJustified     *FinalityCheckpoint

	// Threshold for finality (2/3 of total stake)
	finalityThreshold *big.Int

	// Stats
	finalizedBlocks uint64
	averageFinalityTime time.Duration
}

// NewFinalityEngine creates a new finality engine
func NewFinalityEngine(config *Config, state *BeaconState, attestationPool *AttestationPool) *FinalityEngine {
	return &FinalityEngine{
		config:          config,
		state:           state,
		attestationPool: attestationPool,
		checkpoints:     make(map[uint64]*FinalityCheckpoint),
		finalizedCheckpoints: make([]*FinalityCheckpoint, 0),
	}
}

// ProcessBlock processes a new block for finality
func (fe *FinalityEngine) ProcessBlock(blockHash common.Hash, blockNumber uint64, slot uint64) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	// Create checkpoint
	checkpoint := &FinalityCheckpoint{
		Epoch:       slot / fe.config.SlotsPerEpoch,
		BlockHash:   blockHash,
		BlockNumber: blockNumber,
		Justified:   false,
		Created:     time.Now(),
		TotalStake:  new(big.Int).Set(fe.state.TotalStake),
	}

	// Store checkpoint
	fe.checkpoints[blockNumber] = checkpoint

	log.Debug("Checkpoint created",
		"blockNumber", blockNumber,
		"blockHash", blockHash.Hex()[:10]+"...",
		"epoch", checkpoint.Epoch)

	return nil
}

// CheckFinality checks if a block can be finalized
func (fe *FinalityEngine) CheckFinality(blockHash common.Hash, blockNumber uint64) (bool, error) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	// Get checkpoint
	checkpoint, exists := fe.checkpoints[blockNumber]
	if !exists {
		return false, ErrInvalidCheckpoint
	}

	// Check if already finalized
	if checkpoint.Finalized.After(checkpoint.Created) {
		return true, nil
	}

	// Get attestations for this block
	attestations := fe.attestationPool.GetAttestationsForBlock(blockHash)
	if len(attestations) == 0 {
		log.Debug("No attestations yet", "blockNumber", blockNumber)
		return false, nil
	}

	// Calculate attesting stake
	attestingStake := fe.calculateAttestingStake(attestations)

	// Check if we have 2/3+ stake
	threshold := new(big.Int).Mul(fe.state.TotalStake, big.NewInt(2))
	threshold.Div(threshold, big.NewInt(3))

	if attestingStake.Cmp(threshold) < 0 {
		log.Debug("Insufficient stake for finality",
			"blockNumber", blockNumber,
			"attestingStake", attestingStake,
			"threshold", threshold,
			"attestations", len(attestations))
		return false, nil
	}

	// Check MEV scores - don't finalize blocks with MEV
	avgMEVScore := fe.calculateAverageMEVScore(attestations)
	if avgMEVScore < 80.0 { // Threshold: avg MEV score must be > 80
		log.Warn("Block has low MEV score, delaying finality",
			"blockNumber", blockNumber,
			"avgMEVScore", avgMEVScore)
		return false, nil
	}

	// Check ordering scores
	avgOrderingScore := fe.calculateAverageOrderingScore(attestations)
	if avgOrderingScore < 90.0 { // Threshold: avg ordering score must be > 90
		log.Warn("Block has low ordering score, delaying finality",
			"blockNumber", blockNumber,
			"avgOrderingScore", avgOrderingScore)
		return false, nil
	}

	// Aggregate signatures
	aggSignature := fe.attestationPool.AggregateAttestations(attestations)
	if aggSignature == nil {
		return false, errors.New("failed to aggregate signatures")
	}

	// Update checkpoint
	checkpoint.Justified = true
	checkpoint.Attestations = attestations
	checkpoint.AttestingStake = attestingStake
	checkpoint.AggregateSignature = aggSignature.AggregateSignature
	checkpoint.SignerIndices = fe.extractSignerIndices(attestations)

	// Finalize after justification delay
	now := time.Now()
	justificationAge := now.Sub(checkpoint.Created)

	minAge := time.Duration(fe.config.JustificationDelay) * fe.config.SlotDuration
	if justificationAge >= minAge {
		return fe.finalizeCheckpoint(checkpoint)
	}

	// Justified but not yet finalized
	fe.latestJustified = checkpoint
	log.Info("✅ Block justified",
		"blockNumber", blockNumber,
		"attestingStake", attestingStake,
		"totalStake", fe.state.TotalStake,
		"percentage", float64(attestingStake.Int64())/float64(fe.state.TotalStake.Int64())*100,
		"mevScore", avgMEVScore,
		"orderingScore", avgOrderingScore)

	return false, nil
}

// finalizeCheckpoint finalizes a checkpoint
func (fe *FinalityEngine) finalizeCheckpoint(checkpoint *FinalityCheckpoint) (bool, error) {
	// Check if parent is finalized (for chain consistency)
	if fe.latestFinalized != nil && checkpoint.BlockNumber <= fe.latestFinalized.BlockNumber {
		return false, ErrAlreadyFinalized
	}

	// Mark as finalized
	checkpoint.Finalized = time.Now()

	// Update latest finalized
	fe.latestFinalized = checkpoint
	fe.finalizedCheckpoints = append(fe.finalizedCheckpoints, checkpoint)
	fe.finalizedBlocks++

	// Update state
	fe.state.FinalizedHash = checkpoint.BlockHash

	// Calculate finality time
	finalityTime := checkpoint.Finalized.Sub(checkpoint.Created)
	fe.updateAverageFinalityTime(finalityTime)

	log.Info("🔒 BLOCK FINALIZED",
		"blockNumber", checkpoint.BlockNumber,
		"blockHash", checkpoint.BlockHash.Hex()[:10]+"...",
		"epoch", checkpoint.Epoch,
		"finalityTime", finalityTime,
		"attestations", len(checkpoint.Attestations),
		"attestingStake", checkpoint.AttestingStake,
		"totalStake", checkpoint.TotalStake)

	return true, nil
}

// GetLatestFinalized returns the latest finalized checkpoint
func (fe *FinalityEngine) GetLatestFinalized() *FinalityCheckpoint {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	return fe.latestFinalized
}

// GetLatestJustified returns the latest justified checkpoint
func (fe *FinalityEngine) GetLatestJustified() *FinalityCheckpoint {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	return fe.latestJustified
}

// IsFinalized checks if a block is finalized
func (fe *FinalityEngine) IsFinalized(blockNumber uint64) bool {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	if fe.latestFinalized == nil {
		return false
	}

	return blockNumber <= fe.latestFinalized.BlockNumber
}

// GetFinalityStatus returns finality status
func (fe *FinalityEngine) GetFinalityStatus() *FinalityStatus {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	status := &FinalityStatus{
		TotalFinalized: fe.finalizedBlocks,
		AverageFinalityTime: fe.averageFinalityTime,
	}

	if fe.latestFinalized != nil {
		status.LatestFinalized = fe.latestFinalized.BlockNumber
		status.LatestFinalizedHash = fe.latestFinalized.BlockHash
		status.LatestFinalizedEpoch = fe.latestFinalized.Epoch
	}

	if fe.latestJustified != nil {
		status.LatestJustified = fe.latestJustified.BlockNumber
		status.LatestJustifiedHash = fe.latestJustified.BlockHash
		status.LatestJustifiedEpoch = fe.latestJustified.Epoch
	}

	// Calculate delay
	if fe.latestFinalized != nil && fe.state.Slot > 0 {
		currentBlock := fe.state.Slot
		status.FinalityDelay = currentBlock - fe.latestFinalized.BlockNumber
	}

	return status
}

// Prune removes old checkpoints to save memory
func (fe *FinalityEngine) Prune(keepLast uint64) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	if fe.latestFinalized == nil {
		return
	}

	cutoff := fe.latestFinalized.BlockNumber
	if cutoff > keepLast {
		cutoff -= keepLast
	} else {
		cutoff = 0
	}

	// Remove old checkpoints
	for blockNum := range fe.checkpoints {
		if blockNum < cutoff {
			delete(fe.checkpoints, blockNum)
		}
	}

	log.Debug("Pruned old checkpoints", "cutoff", cutoff, "kept", len(fe.checkpoints))
}

// Helper functions

func (fe *FinalityEngine) calculateAttestingStake(attestations []*Attestation) *big.Int {
	totalStake := big.NewInt(0)
	seen := make(map[common.Address]bool)

	for _, att := range attestations {
		// Skip duplicates
		if seen[att.Validator] {
			continue
		}
		seen[att.Validator] = true

		// Add validator stake
		if validator, exists := fe.state.Validators[att.Validator]; exists {
			totalStake.Add(totalStake, validator.Stake)
		}
	}

	return totalStake
}

func (fe *FinalityEngine) calculateAverageMEVScore(attestations []*Attestation) float64 {
	if len(attestations) == 0 {
		return 100.0
	}

	total := 0.0
	for _, att := range attestations {
		total += att.MEVScore
	}

	return total / float64(len(attestations))
}

func (fe *FinalityEngine) calculateAverageOrderingScore(attestations []*Attestation) float64 {
	if len(attestations) == 0 {
		return 100.0
	}

	total := 0.0
	for _, att := range attestations {
		total += att.OrderingScore
	}

	return total / float64(len(attestations))
}

func (fe *FinalityEngine) extractSignerIndices(attestations []*Attestation) []uint64 {
	indices := make([]uint64, 0, len(attestations))
	for _, att := range attestations {
		indices = append(indices, att.ValidatorIndex)
	}
	return indices
}

func (fe *FinalityEngine) updateAverageFinalityTime(newTime time.Duration) {
	if fe.finalizedBlocks == 1 {
		fe.averageFinalityTime = newTime
		return
	}

	// Calculate running average
	total := fe.averageFinalityTime * time.Duration(fe.finalizedBlocks-1)
	total += newTime
	fe.averageFinalityTime = total / time.Duration(fe.finalizedBlocks)
}

// FinalityStatus holds finality status information
type FinalityStatus struct {
	LatestFinalized     uint64        `json:"latestFinalized"`
	LatestFinalizedHash common.Hash   `json:"latestFinalizedHash"`
	LatestFinalizedEpoch uint64       `json:"latestFinalizedEpoch"`

	LatestJustified     uint64        `json:"latestJustified"`
	LatestJustifiedHash common.Hash   `json:"latestJustifiedHash"`
	LatestJustifiedEpoch uint64       `json:"latestJustifiedEpoch"`

	TotalFinalized      uint64        `json:"totalFinalized"`
	FinalityDelay       uint64        `json:"finalityDelay"`
	AverageFinalityTime time.Duration `json:"averageFinalityTime"`
}
