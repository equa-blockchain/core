// Copyright 2024 The go-equa Authors
// EQUA Beacon Engine - MEV-Aware Attestation System

package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/crypto"
	"github.com/equa/go-equa/log"
)

var (
	ErrInvalidAttestation = errors.New("invalid attestation")
	ErrDuplicateAttestation = errors.New("duplicate attestation")
	ErrAttestationTooOld = errors.New("attestation too old")
)

// AttestationPool manages attestations
type AttestationPool struct {
	mu sync.RWMutex

	// Attestations by slot
	attestations map[uint64][]*Attestation

	// Attestations by validator (to prevent duplicates)
	validatorAttestations map[common.Address]map[uint64]bool

	// Configuration
	maxAge uint64 // Maximum age of attestations in slots

	// RPC client to query block data
	rpc *RPCClient

	// State
	state *BeaconState
}

// NewAttestationPool creates a new attestation pool
func NewAttestationPool(rpc *RPCClient, state *BeaconState) *AttestationPool {
	return &AttestationPool{
		attestations:          make(map[uint64][]*Attestation),
		validatorAttestations: make(map[common.Address]map[uint64]bool),
		maxAge:                64, // Keep last 64 slots
		rpc:                   rpc,
		state:                 state,
	}
}

// AddAttestation adds an attestation to the pool
func (ap *AttestationPool) AddAttestation(att *Attestation) error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	// Validate attestation
	if err := ap.validateAttestation(att); err != nil {
		return err
	}

	// Check for duplicates
	if ap.isDuplicate(att) {
		return ErrDuplicateAttestation
	}

	// Add to pool
	ap.attestations[att.Slot] = append(ap.attestations[att.Slot], att)

	// Track validator attestation
	if ap.validatorAttestations[att.Validator] == nil {
		ap.validatorAttestations[att.Validator] = make(map[uint64]bool)
	}
	ap.validatorAttestations[att.Validator][att.Slot] = true

	log.Debug("Attestation added",
		"slot", att.Slot,
		"validator", att.Validator.Hex()[:10]+"...",
		"blockHash", att.BlockHash.Hex()[:10]+"...",
		"mevScore", att.MEVScore,
		"orderingScore", att.OrderingScore)

	// Clean old attestations
	ap.cleanOldAttestations(att.Slot)

	return nil
}

// CreateAttestation creates an attestation for a block
func (ap *AttestationPool) CreateAttestation(
	slot uint64,
	blockHash common.Hash,
	validator *Validator,
	privateKey []byte,
) (*Attestation, error) {

	// Get block data to assess MEV and ordering
	mevScore, orderingScore, err := ap.assessBlock(blockHash)
	if err != nil {
		log.Warn("Failed to assess block", "error", err)
		// Use default scores if can't assess
		mevScore = 100.0
		orderingScore = 100.0
	}

	// Create attestation
	att := &Attestation{
		Slot:           slot,
		BlockHash:      blockHash,
		ValidatorIndex: ap.state.ValidatorIndices[validator.Address],
		Validator:      validator.Address,
		MEVScore:       mevScore,
		OrderingScore:  orderingScore,
		Timestamp:      time.Now(),
	}

	// Sign attestation
	signature, err := ap.signAttestation(att, privateKey)
	if err != nil {
		return nil, err
	}
	att.Signature = signature

	return att, nil
}

// GetAttestations returns attestations for a slot
func (ap *AttestationPool) GetAttestations(slot uint64) []*Attestation {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	attestations := ap.attestations[slot]
	// Return copy to avoid race conditions
	result := make([]*Attestation, len(attestations))
	copy(result, attestations)

	return result
}

// GetAttestationsForBlock returns all attestations for a specific block
func (ap *AttestationPool) GetAttestationsForBlock(blockHash common.Hash) []*Attestation {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	result := make([]*Attestation, 0)
	for _, atts := range ap.attestations {
		for _, att := range atts {
			if att.BlockHash == blockHash {
				result = append(result, att)
			}
		}
	}

	return result
}

// GetAttestationStats returns statistics about attestations
func (ap *AttestationPool) GetAttestationStats(slot uint64) *AttestationStats {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	atts := ap.attestations[slot]
	if len(atts) == 0 {
		return &AttestationStats{}
	}

	stats := &AttestationStats{
		TotalAttestations: uint64(len(atts)),
	}

	// Calculate average scores
	totalMEV := 0.0
	totalOrdering := 0.0
	uniqueValidators := make(map[common.Address]bool)

	for _, att := range atts {
		totalMEV += att.MEVScore
		totalOrdering += att.OrderingScore
		uniqueValidators[att.Validator] = true
	}

	stats.AverageMEVScore = totalMEV / float64(len(atts))
	stats.AverageOrderingScore = totalOrdering / float64(len(atts))
	stats.UniqueValidators = uint64(len(uniqueValidators))

	// Calculate participation rate
	if ap.state.ActiveValidators > 0 {
		stats.ParticipationRate = float64(stats.UniqueValidators) / float64(ap.state.ActiveValidators)
	}

	return stats
}

// assessBlock assesses a block for MEV and ordering quality
func (ap *AttestationPool) assessBlock(blockHash common.Hash) (mevScore, orderingScore float64, err error) {
	// Get block number from hash
	blockNumber, err := ap.rpc.GetBlockNumberByHash(blockHash)
	if err != nil {
		return 0, 0, err
	}

	// Get MEV score from EQUA consensus
	mevDetected, err := ap.rpc.GetMEVDetected(blockNumber)
	if err != nil {
		log.Warn("Failed to get MEV detection", "error", err)
		mevDetected = false
	}

	// Calculate MEV score (100 = no MEV, 0 = heavy MEV)
	if mevDetected {
		mevScore = 0.0 // MEV detected, worst score
	} else {
		mevScore = 100.0 // No MEV, perfect score
	}

	// Get ordering score from EQUA consensus
	orderingData, err := ap.rpc.GetOrderingScore(blockNumber)
	if err != nil {
		log.Warn("Failed to get ordering score", "error", err)
		orderingScore = 100.0 // Default to perfect if can't assess
	} else {
		// Convert to 0-100 scale
		orderingScore = orderingData.Score * 100.0
	}

	return mevScore, orderingScore, nil
}

// validateAttestation validates an attestation
func (ap *AttestationPool) validateAttestation(att *Attestation) error {
	// Check if validator exists and is active
	validator, exists := ap.state.Validators[att.Validator]
	if !exists {
		return errors.New("validator not found")
	}

	if !validator.Active || validator.Slashed {
		return errors.New("validator not active or slashed")
	}

	// Check attestation age
	if ap.state.Slot > 0 && att.Slot < ap.state.Slot-ap.maxAge {
		return ErrAttestationTooOld
	}

	// Verify signature
	if !ap.verifyAttestationSignature(att, validator.PublicKey) {
		return errors.New("invalid signature")
	}

	// Validate MEV and ordering scores (0-100 range)
	if att.MEVScore < 0 || att.MEVScore > 100 {
		return errors.New("invalid MEV score")
	}

	if att.OrderingScore < 0 || att.OrderingScore > 100 {
		return errors.New("invalid ordering score")
	}

	return nil
}

// isDuplicate checks if attestation is duplicate
func (ap *AttestationPool) isDuplicate(att *Attestation) bool {
	if slots, exists := ap.validatorAttestations[att.Validator]; exists {
		return slots[att.Slot]
	}
	return false
}

// signAttestation signs an attestation
func (ap *AttestationPool) signAttestation(att *Attestation, privateKey []byte) ([]byte, error) {
	// Create message to sign
	msg := ap.attestationSigningMessage(att)

	// Sign with private key (simplified - production would use BLS)
	hash := sha256.Sum256(msg)
	signature := crypto.Keccak256(hash[:], privateKey)

	return signature, nil
}

// verifyAttestationSignature verifies attestation signature
func (ap *AttestationPool) verifyAttestationSignature(att *Attestation, publicKey []byte) bool {
	// Create message
	msg := ap.attestationSigningMessage(att)

	// Verify signature (simplified - production would use BLS verification)
	_ = sha256.Sum256(msg) // Hash computed for future BLS verification

	// For now, just check signature length (placeholder for BLS verification)
	return len(att.Signature) == 32
}

// attestationSigningMessage creates the message to sign
func (ap *AttestationPool) attestationSigningMessage(att *Attestation) []byte {
	msg := make([]byte, 0, 128)

	// Add slot
	slotBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(slotBytes, att.Slot)
	msg = append(msg, slotBytes...)

	// Add block hash
	msg = append(msg, att.BlockHash.Bytes()...)

	// Add validator index
	indexBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(indexBytes, att.ValidatorIndex)
	msg = append(msg, indexBytes...)

	// Add MEV score
	mevBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(mevBytes, uint64(att.MEVScore*1000))
	msg = append(msg, mevBytes...)

	// Add ordering score
	orderingBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(orderingBytes, uint64(att.OrderingScore*1000))
	msg = append(msg, orderingBytes...)

	return msg
}

// cleanOldAttestations removes old attestations
func (ap *AttestationPool) cleanOldAttestations(currentSlot uint64) {
	if currentSlot <= ap.maxAge {
		return
	}

	cutoff := currentSlot - ap.maxAge

	// Remove old attestations
	for slot := range ap.attestations {
		if slot < cutoff {
			delete(ap.attestations, slot)
		}
	}

	// Clean validator attestations
	for validator := range ap.validatorAttestations {
		for slot := range ap.validatorAttestations[validator] {
			if slot < cutoff {
				delete(ap.validatorAttestations[validator], slot)
			}
		}
	}
}

// AggregateAttestations aggregates attestations for a block
func (ap *AttestationPool) AggregateAttestations(attestations []*Attestation) *AggregatedAttestation {
	if len(attestations) == 0 {
		return nil
	}

	agg := &AggregatedAttestation{
		Slot:      attestations[0].Slot,
		BlockHash: attestations[0].BlockHash,
		Attestations: attestations,
	}

	// Calculate aggregate scores
	totalMEV := 0.0
	totalOrdering := 0.0
	validators := make(map[common.Address]bool)

	for _, att := range attestations {
		totalMEV += att.MEVScore
		totalOrdering += att.OrderingScore
		validators[att.Validator] = true
	}

	agg.AggregateMEVScore = totalMEV / float64(len(attestations))
	agg.AggregateOrderingScore = totalOrdering / float64(len(attestations))
	agg.ValidatorCount = uint64(len(validators))

	// Aggregate signatures (simplified - production would use BLS aggregation)
	agg.AggregateSignature = ap.aggregateSignatures(attestations)

	return agg
}

// aggregateSignatures aggregates multiple signatures
func (ap *AttestationPool) aggregateSignatures(attestations []*Attestation) []byte {
	// Simplified aggregation - production would use BLS signature aggregation
	data := make([]byte, 0)
	for _, att := range attestations {
		data = append(data, att.Signature...)
	}

	hash := crypto.Keccak256Hash(data)
	return hash.Bytes()
}

// AttestationStats holds statistics about attestations
type AttestationStats struct {
	TotalAttestations     uint64  `json:"totalAttestations"`
	UniqueValidators      uint64  `json:"uniqueValidators"`
	ParticipationRate     float64 `json:"participationRate"`
	AverageMEVScore       float64 `json:"averageMevScore"`
	AverageOrderingScore  float64 `json:"averageOrderingScore"`
}

// AggregatedAttestation represents aggregated attestations
type AggregatedAttestation struct {
	Slot                   uint64         `json:"slot"`
	BlockHash              common.Hash    `json:"blockHash"`
	Attestations           []*Attestation `json:"attestations"`
	ValidatorCount         uint64         `json:"validatorCount"`
	AggregateMEVScore      float64        `json:"aggregateMevScore"`
	AggregateOrderingScore float64        `json:"aggregateOrderingScore"`
	AggregateSignature     []byte         `json:"aggregateSignature"`
}
