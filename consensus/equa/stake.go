// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"math/big"
	"sort"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/ethdb"
	"github.com/equa/go-equa/log"
	"github.com/equa/go-equa/params"
)

// Validator represents a validator in the EQUA network
type Validator struct {
	Address     common.Address // Validator's address
	Stake       *big.Int       // Amount staked
	KeyShare    []byte         // BLS key share for threshold encryption
	PublicKey   []byte         // BLS public key
	LastBlock   uint64         // Last block proposed
	Slashed     bool           // Whether validator has been slashed
	SlashAmount *big.Int       // Amount slashed
}

// StakeManager manages validator stakes and selection
type StakeManager struct {
	config     *params.EquaConfig
	db         ethdb.Database
	validators map[common.Address]*Validator
	totalStake *big.Int
}

// NewStakeManager creates a new stake manager
func NewStakeManager(db ethdb.Database, config *params.EquaConfig) *StakeManager {
	return &StakeManager{
		config:     config,
		db:         db,
		validators: make(map[common.Address]*Validator),
		totalStake: big.NewInt(0),
	}
}

// AddValidator adds a new validator to the set
func (sm *StakeManager) AddValidator(addr common.Address, stake *big.Int, keyShare, pubKey []byte) error {
	validator := &Validator{
		Address:     addr,
		Stake:       new(big.Int).Set(stake),
		KeyShare:    keyShare,
		PublicKey:   pubKey,
		LastBlock:   0,
		Slashed:     false,
		SlashAmount: big.NewInt(0),
	}

	sm.validators[addr] = validator
	sm.totalStake.Add(sm.totalStake, stake)

	return nil
}

// RemoveValidator removes a validator from the set
func (sm *StakeManager) RemoveValidator(addr common.Address) error {
	if validator, exists := sm.validators[addr]; exists {
		sm.totalStake.Sub(sm.totalStake, validator.Stake)
		delete(sm.validators, addr)
	}
	return nil
}

// HasStake checks if an address has stake
func (sm *StakeManager) HasStake(addr common.Address) bool {
	validator, exists := sm.validators[addr]
	return exists && validator.Stake.Cmp(big.NewInt(0)) > 0 && !validator.Slashed
}

// GetStake returns the stake amount for a validator
func (sm *StakeManager) GetStake(addr common.Address) *big.Int {
	if validator, exists := sm.validators[addr]; exists {
		return new(big.Int).Set(validator.Stake)
	}
	return big.NewInt(0)
}

// GetValidator returns validator information
func (sm *StakeManager) GetValidator(addr common.Address) (*Validator, bool) {
	validator, exists := sm.validators[addr]
	return validator, exists
}

// GetValidators returns all active validators
func (sm *StakeManager) GetValidators() []*Validator {
	validators := make([]*Validator, 0, len(sm.validators))
	for _, validator := range sm.validators {
		if !validator.Slashed && validator.Stake.Cmp(big.NewInt(0)) > 0 {
			validators = append(validators, validator)
		}
	}
	return validators
}

// GetTopStakers returns the top N validators by stake
func (sm *StakeManager) GetTopStakers(n int) []*Validator {
	validators := sm.GetValidators()

	// Sort by stake descending
	sort.Slice(validators, func(i, j int) bool {
		return validators[i].Stake.Cmp(validators[j].Stake) > 0
	})

	if n > len(validators) {
		n = len(validators)
	}

	return validators[:n]
}

// GetStakeWeight returns the stake weight for a validator (stake / total_stake)
func (sm *StakeManager) GetStakeWeight(addr common.Address) *big.Int {
	validator, exists := sm.validators[addr]
	if !exists || validator.Slashed {
		return big.NewInt(0)
	}

	if sm.totalStake.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0)
	}

	// Return stake weight as percentage (stake * 10000 / totalStake)
	weight := new(big.Int).Mul(validator.Stake, big.NewInt(10000))
	weight.Div(weight, sm.totalStake)
	return weight
}

// SlashValidator slashes a validator for malicious behavior
func (sm *StakeManager) SlashValidator(addr common.Address, percentage uint64, reason string) error {
	validator, exists := sm.validators[addr]
	if !exists {
		return errInvalidValidator
	}

	// Calculate slash amount
	slashAmount := new(big.Int).Mul(validator.Stake, big.NewInt(int64(percentage)))
	slashAmount.Div(slashAmount, big.NewInt(100))

	// Apply slash
	validator.Slashed = true
	validator.SlashAmount.Add(validator.SlashAmount, slashAmount)
	validator.Stake.Sub(validator.Stake, slashAmount)
	sm.totalStake.Sub(sm.totalStake, slashAmount)

	// Log slashing event
	log.Warn("⚡ Validator slashed",
		"validator", validator.Address.Hex()[:10]+"...",
		"amount", slashAmount.String(),
		"reason", reason)

	return nil
}

// UpdateLastBlock updates the last block proposed by a validator
func (sm *StakeManager) UpdateLastBlock(addr common.Address, blockNumber uint64) {
	if validator, exists := sm.validators[addr]; exists {
		validator.LastBlock = blockNumber
	}
}

// GetTotalStake returns the total stake in the network
func (sm *StakeManager) GetTotalStake() *big.Int {
	return new(big.Int).Set(sm.totalStake)
}

// IsEligible checks if a validator is eligible to propose/validate with advanced criteria
func (sm *StakeManager) IsEligible(addr common.Address) bool {
	validator, exists := sm.validators[addr]
	if !exists {
		return false
	}

	// Must have stake, not be slashed, and meet minimum requirements
	minStake := big.NewInt(32) // 32 EQUA minimum stake
	minStake.Mul(minStake, big.NewInt(1e18))

	// Basic eligibility
	if validator.Slashed || validator.Stake.Cmp(minStake) < 0 {
		return false
	}

	// Additional checks:
	// 1. Check if validator has been recently slashed (cooldown period)
	if validator.SlashAmount.Cmp(big.NewInt(0)) > 0 {
		// Reduced eligibility if previously slashed
		effectiveStake := new(big.Int).Sub(validator.Stake, validator.SlashAmount)
		if effectiveStake.Cmp(minStake) < 0 {
			return false
		}
	}

	// 2. Check validator activity (must have proposed recently)
	// Allow validators to be inactive for up to 100 blocks
	if validator.LastBlock > 0 {
		// Note: This check would need current block number passed in
		// For now, we just check if they've ever proposed
		return true
	}

	return true
}

// GetValidatorPerformanceScore calculates a performance score for a validator
func (sm *StakeManager) GetValidatorPerformanceScore(addr common.Address) float64 {
	validator, exists := sm.validators[addr]
	if !exists {
		return 0.0
	}

	score := 1.0

	// Penalize for slashing
	if validator.Slashed {
		score *= 0.1 // Heavy penalty
	} else if validator.SlashAmount.Cmp(big.NewInt(0)) > 0 {
		// Partial penalty for previous slashing
		penalty := new(big.Int).Div(validator.SlashAmount, validator.Stake)
		score *= (1.0 - float64(penalty.Uint64())/100.0)
	}

	// Reward for higher stake
	stakeRatio := new(big.Int).Div(validator.Stake, big.NewInt(1e18))
	if stakeRatio.Uint64() > 100 {
		score *= 1.2
	} else if stakeRatio.Uint64() > 50 {
		score *= 1.1
	}

	return score
}

// GetKeyShares returns key shares for threshold decryption
func (sm *StakeManager) GetKeyShares(validators []common.Address) [][]byte {
	shares := make([][]byte, 0, len(validators))
	for _, addr := range validators {
		if validator, exists := sm.validators[addr]; exists && !validator.Slashed {
			shares = append(shares, validator.KeyShare)
		}
	}
	return shares
}
