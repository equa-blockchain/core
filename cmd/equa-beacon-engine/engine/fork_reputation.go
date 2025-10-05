// Copyright 2024 The go-equa Authors
// EQUA Beacon Engine - MEV-Aware Fork Choice & Reputation System

package engine

import (
	"math/big"
	"sync"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/log"
)

// ForkChoice implements MEV-aware fork choice rule
type ForkChoice struct {
	mu sync.RWMutex

	state *BeaconState
	rpc   *RPCClient

	// Forks being tracked
	forks map[common.Hash]*Fork

	// Head of canonical chain
	head common.Hash

	// Configuration
	mevPenaltyFactor     float64 // Penalty multiplier for MEV
	orderingBonusFactor  float64 // Bonus multiplier for fair ordering
}

// NewForkChoice creates MEV-aware fork choice
func NewForkChoice(state *BeaconState, rpc *RPCClient) *ForkChoice {
	return &ForkChoice{
		state:                state,
		rpc:                  rpc,
		forks:                make(map[common.Hash]*Fork),
		mevPenaltyFactor:     0.5,  // 50% penalty
		orderingBonusFactor:  1.1,  // 10% bonus
	}
}

// AddBlock adds block to fork choice
func (fc *ForkChoice) AddBlock(blockHash common.Hash, blockNumber uint64, parentHash common.Hash) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Get or create fork
	fork, exists := fc.forks[blockHash]
	if !exists {
		fork = &Fork{
			Head:       blockHash,
			Height:     blockNumber,
			TotalStake: big.NewInt(0),
			MEVPenalty: big.NewInt(0),
			OrderingBonus: big.NewInt(0),
			LastUpdated: time.Now(),
		}
		fc.forks[blockHash] = fork
	}

	// Update fork weight based on MEV and ordering
	if err := fc.updateForkWeight(fork, blockHash, blockNumber); err != nil {
		log.Warn("Failed to update fork weight", "error", err)
	}

	// Choose canonical head
	fc.chooseHead()

	return nil
}

// updateForkWeight calculates fork weight with MEV penalties
func (fc *ForkChoice) updateForkWeight(fork *Fork, blockHash common.Hash, blockNumber uint64) error {
	// Base weight = total validator stake
	fork.TotalStake = new(big.Int).Set(fc.state.TotalStake)

	// Check for MEV
	mevDetected, err := fc.rpc.GetMEVDetected(blockNumber)
	if err == nil && mevDetected {
		// Apply MEV penalty
		penalty := new(big.Int).Mul(fork.TotalStake, big.NewInt(int64(fc.mevPenaltyFactor*100)))
		penalty.Div(penalty, big.NewInt(100))
		fork.MEVPenalty = penalty
	}

	// Check ordering score
	orderingData, err := fc.rpc.GetOrderingScore(blockNumber)
	if err == nil && orderingData.FairOrdering {
		// Apply ordering bonus
		bonus := new(big.Int).Mul(fork.TotalStake, big.NewInt(int64((fc.orderingBonusFactor-1.0)*100)))
		bonus.Div(bonus, big.NewInt(100))
		fork.OrderingBonus = bonus
	}

	// Calculate effective weight
	fork.EffectiveWeight = new(big.Int).Set(fork.TotalStake)
	fork.EffectiveWeight.Sub(fork.EffectiveWeight, fork.MEVPenalty)
	fork.EffectiveWeight.Add(fork.EffectiveWeight, fork.OrderingBonus)

	return nil
}

// chooseHead selects canonical chain head
func (fc *ForkChoice) chooseHead() {
	var bestFork *Fork
	var bestHash common.Hash

	for hash, fork := range fc.forks {
		if bestFork == nil || fork.EffectiveWeight.Cmp(bestFork.EffectiveWeight) > 0 {
			bestFork = fork
			bestHash = hash
		} else if fork.EffectiveWeight.Cmp(bestFork.EffectiveWeight) == 0 {
			// Tie-breaker: choose higher block
			if fork.Height > bestFork.Height {
				bestFork = fork
				bestHash = hash
			}
		}
	}

	if bestHash != fc.head {
		oldHead := fc.head
		fc.head = bestHash
		fc.state.LatestBlockHash = bestHash

		log.Info("🔀 Fork choice updated",
			"oldHead", oldHead.Hex()[:10]+"...",
			"newHead", bestHash.Hex()[:10]+"...",
			"height", bestFork.Height,
			"effectiveWeight", bestFork.EffectiveWeight)
	}
}

// GetHead returns canonical chain head
func (fc *ForkChoice) GetHead() common.Hash {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.head
}

// ReputationManager manages validator reputation
type ReputationManager struct {
	mu sync.RWMutex

	state *BeaconState
	rpc   *RPCClient

	// Reputation tracking
	reputations map[common.Address]*Reputation

	// Configuration
	decayRate        float64 // How fast reputation decays
	updateInterval   time.Duration
}

// NewReputationManager creates reputation manager
func NewReputationManager(state *BeaconState, rpc *RPCClient) *ReputationManager {
	return &ReputationManager{
		state:          state,
		rpc:            rpc,
		reputations:    make(map[common.Address]*Reputation),
		decayRate:      0.01, // 1% decay per epoch
		updateInterval: 1 * time.Hour,
	}
}

// UpdateReputation updates validator reputation based on behavior
func (rm *ReputationManager) UpdateReputation(validator common.Address, blockProposed bool, mevDetected bool, orderingScore float64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rep, exists := rm.reputations[validator]
	if !exists {
		rep = &Reputation{
			MEVScore:      100.0,
			OrderingScore: 100.0,
			UptimeScore:   100.0,
			AttestationRate: 1.0,
			OverallScore:  100.0,
			LastUpdated:   time.Now(),
		}
		rm.reputations[validator] = rep
	}

	if blockProposed {
		rep.TotalBlocks++

		// Update MEV score
		if mevDetected {
			rep.BlocksWithMEV++
			rep.MEVScore = max(0, rep.MEVScore-10.0) // -10 for MEV
		} else {
			rep.MEVScore = min(100, rep.MEVScore+1.0) // +1 for clean block
		}

		// Update ordering score
		rep.OrderingScore = (rep.OrderingScore*0.9 + orderingScore*0.1)
	}

	// Update overall score (weighted average)
	rep.OverallScore = (rep.MEVScore*0.4 + rep.OrderingScore*0.3 +
		rep.UptimeScore*0.2 + rep.AttestationRate*100*0.1)

	rep.LastUpdated = time.Now()

	// Update state
	if v, exists := rm.state.Validators[validator]; exists {
		v.Reputation = rep
	}
}

// GetReputation returns validator reputation
func (rm *ReputationManager) GetReputation(validator common.Address) *Reputation {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rep, exists := rm.reputations[validator]; exists {
		return rep
	}

	// Return default perfect reputation
	return &Reputation{
		MEVScore:      100.0,
		OrderingScore: 100.0,
		UptimeScore:   100.0,
		AttestationRate: 1.0,
		OverallScore:  100.0,
	}
}

// ApplyDecay applies reputation decay over time
func (rm *ReputationManager) ApplyDecay() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	for _, rep := range rm.reputations {
		// Decay all scores slightly
		rep.MEVScore = max(50, rep.MEVScore*(1.0-rm.decayRate))
		rep.OrderingScore = max(50, rep.OrderingScore*(1.0-rm.decayRate))
		rep.UptimeScore = max(50, rep.UptimeScore*(1.0-rm.decayRate))

		// Recalculate overall
		rep.OverallScore = (rep.MEVScore*0.4 + rep.OrderingScore*0.3 +
			rep.UptimeScore*0.2 + rep.AttestationRate*100*0.1)
	}
}

// RewardCalculator calculates validator rewards
type RewardCalculator struct {
	config *Config
	state  *BeaconState
	repMgr *ReputationManager
}

// NewRewardCalculator creates reward calculator
func NewRewardCalculator(config *Config, state *BeaconState, repMgr *ReputationManager) *RewardCalculator {
	return &RewardCalculator{
		config: config,
		state:  state,
		repMgr: repMgr,
	}
}

// CalculateReward calculates reward for validator
func (rc *RewardCalculator) CalculateReward(validator common.Address, blockProduced bool, mevDetected bool, orderingScore float64) *big.Int {
	baseReward := new(big.Int).Set(rc.config.BaseRewardPerEpoch)

	// No reward if no block produced
	if !blockProduced {
		return big.NewInt(0)
	}

	// Get reputation
	rep := rc.repMgr.GetReputation(validator)

	// Calculate multiplier based on behavior
	multiplier := 1.0

	// MEV bonus/penalty
	if !mevDetected {
		multiplier += rc.config.MEVBonusMultiplier // +20% for no MEV
	} else {
		multiplier -= 0.5 // -50% penalty for MEV
	}

	// Ordering bonus
	if orderingScore > 0.95 {
		multiplier += rc.config.OrderingBonusMultiplier // +15% for fair ordering
	}

	// Reputation bonus
	if rep.OverallScore > 90 {
		multiplier += 0.1 // +10% for high reputation
	}

	// Apply multiplier
	reward := new(big.Int).Mul(baseReward, big.NewInt(int64(multiplier*100)))
	reward.Div(reward, big.NewInt(100))

	return reward
}

// Helper functions
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
