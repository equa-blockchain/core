// Copyright 2024 The go-equa Authors
// EQUA Beacon Engine - Hybrid PoW+PoS Proposer Selection

package engine

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/big"
	"sort"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/crypto"
	"github.com/equa/go-equa/log"
)

var (
	ErrNoValidators = errors.New("no active validators")
	ErrInvalidSlot  = errors.New("invalid slot number")
)

// ProposerSelector handles hybrid PoW+PoS proposer selection
type ProposerSelector struct {
	config     *Config
	state      *BeaconState
	rpc        *RPCClient

	// VRF state
	vrfPrivateKey []byte
	vrfPublicKey  []byte

	// Cache
	cachedSelections map[uint64]*ProposerSelectionResult
}

// NewProposerSelector creates a new proposer selector
func NewProposerSelector(config *Config, state *BeaconState, rpc *RPCClient) *ProposerSelector {
	// Generate VRF keys
	privKey, pubKey := generateVRFKeys()

	return &ProposerSelector{
		config:           config,
		state:            state,
		rpc:              rpc,
		vrfPrivateKey:    privKey,
		vrfPublicKey:     pubKey,
		cachedSelections: make(map[uint64]*ProposerSelectionResult),
	}
}

// SelectProposer selects the proposer for a given slot using hybrid PoW+PoS
func (ps *ProposerSelector) SelectProposer(slot uint64) (*ProposerSelectionResult, error) {
	startTime := time.Now()

	// Check cache first
	if cached, exists := ps.cachedSelections[slot]; exists {
		return cached, nil
	}

	// Get active validators
	validators := ps.getActiveValidators()
	if len(validators) == 0 {
		return nil, ErrNoValidators
	}

	// Get PoW quality from latest block
	powQuality, err := ps.getPoWQuality()
	if err != nil {
		log.Warn("Failed to get PoW quality, using fallback", "error", err)
		powQuality = big.NewInt(1)
	}

	// Generate selection seed (deterministic but unpredictable)
	seed := ps.generateSelectionSeed(slot, powQuality)

	// Calculate weights for each validator (PoW + PoS hybrid)
	weights := ps.calculateValidatorWeights(validators, powQuality, seed)

	// Select proposer using weighted VRF
	proposer, vrfOutput, vrfProof := ps.weightedVRFSelection(validators, weights, seed)

	// Create result
	result := &ProposerSelectionResult{
		Slot:          slot,
		Proposer:      proposer.Address,
		PoWQuality:    powQuality,
		StakeWeight:   proposer.Stake,
		VRFOutput:     vrfOutput,
		VRFProof:      vrfProof,
		SelectionSeed: seed,
		Timestamp:     time.Now(),
		SelectionTime: time.Since(startTime),
	}

	// Cache result
	ps.cachedSelections[slot] = result

	// Clean old cache entries (keep last 100 slots)
	if len(ps.cachedSelections) > 100 {
		ps.cleanCache(slot)
	}

	log.Debug("Proposer selected",
		"slot", slot,
		"proposer", proposer.Address.Hex()[:10]+"...",
		"stake", proposer.Stake,
		"powQuality", powQuality,
		"selectionTime", result.SelectionTime)

	return result, nil
}

// getActiveValidators returns all active validators sorted by address
func (ps *ProposerSelector) getActiveValidators() []*Validator {
	validators := make([]*Validator, 0, len(ps.state.Validators))

	for _, v := range ps.state.Validators {
		if v.Active && !v.Slashed && v.Stake.Cmp(ps.config.MinStake) >= 0 {
			// Check reputation threshold
			if v.Reputation != nil && v.Reputation.OverallScore >= ps.config.MinReputationScore {
				validators = append(validators, v)
			}
		}
	}

	// Sort by address for deterministic ordering
	sort.Slice(validators, func(i, j int) bool {
		return validators[i].Address.Hex() < validators[j].Address.Hex()
	})

	return validators
}

// getPoWQuality fetches the PoW quality from the latest block
func (ps *ProposerSelector) getPoWQuality() (*big.Int, error) {
	// Call EQUA RPC to get latest block's PoW quality
	quality, err := ps.rpc.GetPoWQuality()
	if err != nil {
		return nil, err
	}

	return quality, nil
}

// generateSelectionSeed creates a deterministic but unpredictable seed
func (ps *ProposerSelector) generateSelectionSeed(slot uint64, powQuality *big.Int) common.Hash {
	// Combine slot number, PoW quality, and epoch seed
	data := make([]byte, 0, 32+32+8)

	// Add PoW quality
	powBytes := make([]byte, 32)
	powQuality.FillBytes(powBytes)
	data = append(data, powBytes...)

	// Add slot number
	slotBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(slotBytes, slot)
	data = append(data, slotBytes...)

	// Add epoch seed if available
	if ps.state.CurrentEpoch != nil {
		data = append(data, ps.state.CurrentEpoch.PoWSeed.Bytes()...)
	}

	return crypto.Keccak256Hash(data)
}

// calculateValidatorWeights calculates hybrid PoW+PoS weights for validators
func (ps *ProposerSelector) calculateValidatorWeights(validators []*Validator, powQuality *big.Int, seed common.Hash) map[common.Address]*big.Int {
	weights := make(map[common.Address]*big.Int)

	// PoW influence factor (0-1)
	powInfluence := ps.config.PoWInfluence
	posInfluence := 1.0 - powInfluence

	for _, v := range validators {
		// Base weight from stake (PoS component)
		stakeWeight := new(big.Int).Set(v.Stake)

		// Apply PoS influence
		posComponent := new(big.Int).Mul(stakeWeight, big.NewInt(int64(posInfluence*1000)))
		posComponent.Div(posComponent, big.NewInt(1000))

		// Calculate PoW component (based on PoW quality)
		// Validators with better reputation get more PoW influence
		reputationMultiplier := 1.0
		if v.Reputation != nil {
			reputationMultiplier = v.Reputation.OverallScore / 100.0
		}

		powComponent := new(big.Int).Mul(powQuality, big.NewInt(int64(powInfluence*reputationMultiplier*1000)))
		powComponent.Div(powComponent, big.NewInt(1000))

		// Combine PoS and PoW components
		totalWeight := new(big.Int).Add(posComponent, powComponent)

		// Apply reputation modifier
		if v.Reputation != nil {
			// Boost for high reputation (up to 20% bonus)
			if v.Reputation.OverallScore > 90 {
				bonus := new(big.Int).Mul(totalWeight, big.NewInt(20))
				bonus.Div(bonus, big.NewInt(100))
				totalWeight.Add(totalWeight, bonus)
			}
			// Penalty for low reputation (up to 50% penalty)
			if v.Reputation.OverallScore < 70 {
				penalty := new(big.Int).Mul(totalWeight, big.NewInt(50))
				penalty.Div(penalty, big.NewInt(100))
				totalWeight.Sub(totalWeight, penalty)
			}
		}

		// Ensure minimum weight
		if totalWeight.Cmp(big.NewInt(1)) < 0 {
			totalWeight = big.NewInt(1)
		}

		weights[v.Address] = totalWeight
	}

	return weights
}

// weightedVRFSelection performs weighted random selection using VRF
func (ps *ProposerSelector) weightedVRFSelection(validators []*Validator, weights map[common.Address]*big.Int, seed common.Hash) (*Validator, []byte, []byte) {
	// Calculate total weight
	totalWeight := big.NewInt(0)
	for _, weight := range weights {
		totalWeight.Add(totalWeight, weight)
	}

	// Generate VRF output and proof
	vrfOutput, vrfProof := ps.generateVRF(seed.Bytes())

	// Convert VRF output to selection index
	vrfValue := new(big.Int).SetBytes(vrfOutput)
	vrfValue.Mod(vrfValue, totalWeight)

	// Find validator at the selected position
	cumulative := big.NewInt(0)
	for _, v := range validators {
		weight := weights[v.Address]
		cumulative.Add(cumulative, weight)

		if cumulative.Cmp(vrfValue) > 0 {
			return v, vrfOutput, vrfProof
		}
	}

	// Fallback (should never happen)
	return validators[0], vrfOutput, vrfProof
}

// generateVRF generates VRF output and proof
// In production, this would use proper VRF (like ECVRF)
func (ps *ProposerSelector) generateVRF(seed []byte) (output []byte, proof []byte) {
	// Simplified VRF for now
	// Production should use: github.com/coniks-sys/coniks-go/crypto/vrf

	// Combine seed with private key
	data := append(seed, ps.vrfPrivateKey...)

	// Generate output
	hash := sha256.Sum256(data)
	output = hash[:]

	// Generate proof (simplified - in production use proper VRF proof)
	proofHash := sha256.Sum256(append(output, ps.vrfPublicKey...))
	proof = proofHash[:]

	return output, proof
}

// VerifyVRF verifies a VRF output and proof
func (ps *ProposerSelector) VerifyVRF(seed []byte, output []byte, proof []byte, publicKey []byte) bool {
	// Simplified verification
	// Production should use proper VRF verification

	expectedProof := sha256.Sum256(append(output, publicKey...))
	return string(proof) == string(expectedProof[:])
}

// ScheduleEpoch pre-schedules proposers for an entire epoch
func (ps *ProposerSelector) ScheduleEpoch(epoch uint64) ([]common.Address, error) {
	startSlot := epoch * ps.config.SlotsPerEpoch
	endSlot := startSlot + ps.config.SlotsPerEpoch

	schedule := make([]common.Address, 0, ps.config.SlotsPerEpoch)

	for slot := startSlot; slot < endSlot; slot++ {
		result, err := ps.SelectProposer(slot)
		if err != nil {
			return nil, err
		}
		schedule = append(schedule, result.Proposer)
	}

	log.Info("📅 Epoch scheduled",
		"epoch", epoch,
		"slots", len(schedule),
		"uniqueProposers", countUnique(schedule))

	return schedule, nil
}

// GetProposer returns the cached proposer for a slot or selects a new one
func (ps *ProposerSelector) GetProposer(slot uint64) (common.Address, error) {
	result, err := ps.SelectProposer(slot)
	if err != nil {
		return common.Address{}, err
	}
	return result.Proposer, nil
}

// cleanCache removes old cache entries
func (ps *ProposerSelector) cleanCache(currentSlot uint64) {
	for slot := range ps.cachedSelections {
		if slot < currentSlot-100 {
			delete(ps.cachedSelections, slot)
		}
	}
}

// Helper functions

func generateVRFKeys() (privateKey []byte, publicKey []byte) {
	// Generate random private key
	privateKey = make([]byte, 32)
	rand.Read(privateKey)

	// Derive public key (simplified - production would use proper EC)
	hash := sha256.Sum256(privateKey)
	publicKey = hash[:]

	return privateKey, publicKey
}

func countUnique(addresses []common.Address) int {
	seen := make(map[common.Address]bool)
	for _, addr := range addresses {
		seen[addr] = true
	}
	return len(seen)
}
