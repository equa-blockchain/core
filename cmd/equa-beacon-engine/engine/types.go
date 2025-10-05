// Copyright 2024 The go-equa Authors
// EQUA Beacon Engine - Core Types

package engine

import (
	"math/big"
	"time"

	"github.com/equa/go-equa/common"
)

// Validator represents a validator in the EQUA network
type Validator struct {
	Address       common.Address `json:"address"`
	Stake         *big.Int       `json:"stake"`
	PublicKey     []byte         `json:"publicKey"`
	Active        bool           `json:"active"`
	Slashed       bool           `json:"slashed"`
	LastProposed  uint64         `json:"lastProposed"`
	Reputation    *Reputation    `json:"reputation"`
	JoinedEpoch   uint64         `json:"joinedEpoch"`
	ExitEpoch     uint64         `json:"exitEpoch,omitempty"`
}

// Reputation tracks validator behavior and performance
type Reputation struct {
	MEVScore        float64 `json:"mevScore"`        // 0-100 (100 = no MEV)
	OrderingScore   float64 `json:"orderingScore"`   // 0-100 (100 = perfect FCFS)
	UptimeScore     float64 `json:"uptimeScore"`     // 0-100 (100 = always online)
	AttestationRate float64 `json:"attestationRate"` // 0-1 (1 = 100% participation)
	OverallScore    float64 `json:"overallScore"`    // Weighted average

	// Historical stats
	TotalBlocks         uint64 `json:"totalBlocks"`
	BlocksWithMEV       uint64 `json:"blocksWithMEV"`
	MissedAttestations  uint64 `json:"missedAttestations"`
	TotalAttestations   uint64 `json:"totalAttestations"`

	// Timestamps
	LastUpdated time.Time `json:"lastUpdated"`
}

// Attestation represents a validator's vote on a block
type Attestation struct {
	Slot           uint64         `json:"slot"`
	BlockHash      common.Hash    `json:"blockHash"`
	ValidatorIndex uint64         `json:"validatorIndex"`
	Validator      common.Address `json:"validator"`

	// EQUA-specific: MEV and ordering attestation
	MEVScore      float64 `json:"mevScore"`      // Validator's assessment of MEV in block
	OrderingScore float64 `json:"orderingScore"` // Validator's assessment of ordering fairness

	// Signature
	Signature []byte `json:"signature"`

	// Metadata
	Timestamp time.Time `json:"timestamp"`
}

// Slot represents a time slot for block production
type Slot struct {
	Number    uint64         `json:"number"`
	Epoch     uint64         `json:"epoch"`
	Proposer  common.Address `json:"proposer"`
	Timestamp uint64         `json:"timestamp"`

	// EQUA-specific: PoW-based selection
	PoWQuality *big.Int    `json:"powQuality"`
	VRFSeed    common.Hash `json:"vrfSeed"`
}

// Epoch represents a collection of slots
type Epoch struct {
	Number           uint64           `json:"number"`
	StartSlot        uint64           `json:"startSlot"`
	EndSlot          uint64           `json:"endSlot"`
	Validators       []common.Address `json:"validators"`
	TotalStake       *big.Int         `json:"totalStake"`
	ProposerSchedule []common.Address `json:"proposerSchedule"`

	// Finality info
	Finalized       bool        `json:"finalized"`
	FinalizedHash   common.Hash `json:"finalizedHash,omitempty"`
	JustifiedHash   common.Hash `json:"justifiedHash,omitempty"`

	// EQUA-specific
	PoWSeed     common.Hash `json:"powSeed"`     // Combined PoW from epoch
	ReputationSnapshot map[common.Address]*Reputation `json:"reputationSnapshot"`
}

// FinalityCheckpoint represents a finality checkpoint
type FinalityCheckpoint struct {
	Epoch      uint64      `json:"epoch"`
	BlockHash  common.Hash `json:"blockHash"`
	BlockNumber uint64     `json:"blockNumber"`

	// Justification info
	Justified  bool                `json:"justified"`
	Attestations []*Attestation    `json:"attestations"`

	// Threshold signature
	AggregateSignature []byte `json:"aggregateSignature"`
	SignerIndices      []uint64 `json:"signerIndices"`

	// Stake info
	AttestingStake *big.Int `json:"attestingStake"`
	TotalStake     *big.Int `json:"totalStake"`

	// Timestamps
	Created   time.Time `json:"created"`
	Finalized time.Time `json:"finalized,omitempty"`
}

// ProposerSelectionResult contains the result of proposer selection
type ProposerSelectionResult struct {
	Slot         uint64         `json:"slot"`
	Proposer     common.Address `json:"proposer"`
	PoWQuality   *big.Int       `json:"powQuality"`
	StakeWeight  *big.Int       `json:"stakeWeight"`
	VRFOutput    []byte         `json:"vrfOutput"`
	VRFProof     []byte         `json:"vrfProof"`
	SelectionSeed common.Hash   `json:"selectionSeed"`

	// Metadata
	Timestamp     time.Time `json:"timestamp"`
	SelectionTime time.Duration `json:"selectionTime"`
}

// Fork represents a competing chain
type Fork struct {
	Head        common.Hash `json:"head"`
	Height      uint64      `json:"height"`
	TotalStake  *big.Int    `json:"totalStake"`

	// EQUA-specific: MEV-aware fork choice
	MEVPenalty      *big.Int `json:"mevPenalty"`
	OrderingBonus   *big.Int `json:"orderingBonus"`
	EffectiveWeight *big.Int `json:"effectiveWeight"`

	// Fork info
	LastUpdated time.Time `json:"lastUpdated"`
}

// BeaconState represents the current state of the beacon chain
type BeaconState struct {
	Slot            uint64 `json:"slot"`
	Epoch           uint64 `json:"epoch"`

	// Chain info
	GenesisTime     uint64      `json:"genesisTime"`
	LatestBlockHash common.Hash `json:"latestBlockHash"`
	FinalizedHash   common.Hash `json:"finalizedHash"`
	JustifiedHash   common.Hash `json:"justifiedHash"`

	// Validators
	Validators       map[common.Address]*Validator `json:"validators"`
	ValidatorIndices map[common.Address]uint64     `json:"validatorIndices"`
	TotalStake       *big.Int                      `json:"totalStake"`
	ActiveValidators uint64                        `json:"activeValidators"`

	// Current epoch info
	CurrentEpoch      *Epoch `json:"currentEpoch"`
	ProposerSchedule  []common.Address `json:"proposerSchedule"`

	// Attestations
	PendingAttestations []*Attestation `json:"pendingAttestations"`

	// Finality
	FinalityCheckpoints map[uint64]*FinalityCheckpoint `json:"finalityCheckpoints"`

	// Fork choice
	Forks map[common.Hash]*Fork `json:"forks"`

	// Metadata
	LastUpdated time.Time `json:"lastUpdated"`
}

// Config holds EQUA Beacon Engine configuration
type Config struct {
	// Network
	NetworkID       uint64 `json:"networkId"`
	ChainID         uint64 `json:"chainId"`

	// Timing
	SlotDuration    time.Duration `json:"slotDuration"`    // Time per slot (e.g., 12s)
	SlotsPerEpoch   uint64        `json:"slotsPerEpoch"`   // Slots in an epoch

	// Consensus
	MinValidators        uint64  `json:"minValidators"`        // Minimum active validators
	MinStake             *big.Int `json:"minStake"`            // Minimum stake to be validator
	MaxValidators        uint64  `json:"maxValidators"`        // Maximum validators

	// Finality
	FinalityThreshold    float64 `json:"finalityThreshold"`    // % of stake needed (e.g., 0.67 = 2/3)
	JustificationDelay   uint64  `json:"justificationDelay"`   // Slots before justification
	FinalizationDelay    uint64  `json:"finalizationDelay"`    // Slots before finalization

	// Rewards
	BaseRewardPerEpoch   *big.Int `json:"baseRewardPerEpoch"`   // Base reward
	MEVBonusMultiplier   float64  `json:"mevBonusMultiplier"`   // Bonus for no-MEV blocks
	OrderingBonusMultiplier float64 `json:"orderingBonusMultiplier"` // Bonus for fair ordering

	// Slashing
	SlashingPenalty      float64 `json:"slashingPenalty"`      // % of stake slashed
	InactivityPenalty    float64 `json:"inactivityPenalty"`    // Penalty for missing attestations

	// PoW Integration
	PoWInfluence         float64 `json:"powInfluence"`         // Weight of PoW in selection (0-1)
	MinPoWQuality        uint64  `json:"minPowQuality"`        // Minimum PoW quality

	// Reputation
	ReputationDecayRate  float64 `json:"reputationDecayRate"`  // How fast reputation decays
	MinReputationScore   float64 `json:"minReputationScore"`   // Min score to propose

	// P2P
	MaxPeers            int    `json:"maxPeers"`
	BootstrapNodes      []string `json:"bootstrapNodes"`

	// API Endpoints
	ExecutionEndpoint   string `json:"executionEndpoint"`
	JWTSecretPath       string `json:"jwtSecretPath"`
	RPCEndpoint         string `json:"rpcEndpoint"`

	// Validator
	ValidatorAddress    string `json:"validatorAddress"`
	ValidatorPrivateKey string `json:"validatorPrivateKey,omitempty"`
}

// Stats holds beacon engine statistics
type Stats struct {
	// Performance
	SlotsProcessed     uint64        `json:"slotsProcessed"`
	BlocksProposed     uint64        `json:"blocksProposed"`
	AttestationsSent   uint64        `json:"attestationsSent"`
	MissedSlots        uint64        `json:"missedSlots"`

	// Timing
	AverageSlotTime    time.Duration `json:"averageSlotTime"`
	LastSlotTime       time.Duration `json:"lastSlotTime"`

	// Fork choice
	Reorganizations    uint64 `json:"reorganizations"`
	CurrentForkDepth   uint64 `json:"currentForkDepth"`

	// Finality
	LastFinalizedEpoch uint64 `json:"lastFinalizedEpoch"`
	FinalityDelay      uint64 `json:"finalityDelay"` // Slots behind finality

	// Attestations
	AttestationSuccessRate float64 `json:"attestationSuccessRate"`
	AverageAttestationTime time.Duration `json:"averageAttestationTime"`

	// Reputation
	AverageNetworkReputation float64 `json:"averageNetworkReputation"`

	// Uptime
	StartTime   time.Time     `json:"startTime"`
	Uptime      time.Duration `json:"uptime"`

	// Sync
	IsSyncing   bool   `json:"isSyncing"`
	SyncProgress float64 `json:"syncProgress"`
}
