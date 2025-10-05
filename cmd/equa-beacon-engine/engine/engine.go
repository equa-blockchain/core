// Copyright 2024 The go-equa Authors
// EQUA Beacon Engine - Main Coordinator

package engine

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/common/hexutil"
	"github.com/equa/go-equa/log"
)

var (
	ErrNotProposer = errors.New("not selected as proposer for this slot")
	ErrEngineStopped = errors.New("engine is stopped")
)

// Engine is the main EQUA Beacon Engine
type Engine struct {
	mu sync.RWMutex

	// Configuration
	config *Config

	// State
	state *BeaconState

	// Components
	rpc               *RPCClient
	proposerSelector  *ProposerSelector
	attestationPool   *AttestationPool
	finalityEngine    *FinalityEngine
	forkChoice        *ForkChoice
	reputationManager *ReputationManager
	rewardCalculator  *RewardCalculator

	// Validator info
	validatorAddress common.Address
	validatorPrivateKey []byte

	// Runtime
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Stats
	stats *Stats

	// Channels
	newSlotCh chan uint64
}

// NewEngine creates a new EQUA Beacon Engine
func NewEngine(config *Config) (*Engine, error) {
	// Create RPC client
	rpc := NewRPCClient(config.ExecutionEndpoint, config.RPCEndpoint, config.JWTSecretPath)

	// Initialize state
	state := &BeaconState{
		Slot:                 0,
		Epoch:                0,
		GenesisTime:          uint64(time.Now().Unix()),
		Validators:           make(map[common.Address]*Validator),
		ValidatorIndices:     make(map[common.Address]uint64),
		TotalStake:           big.NewInt(0),
		PendingAttestations:  make([]*Attestation, 0),
		FinalityCheckpoints:  make(map[uint64]*FinalityCheckpoint),
		Forks:                make(map[common.Hash]*Fork),
		LastUpdated:          time.Now(),
	}

	// Create components
	attestationPool := NewAttestationPool(rpc, state)
	proposerSelector := NewProposerSelector(config, state, rpc)
	finalityEngine := NewFinalityEngine(config, state, attestationPool)
	forkChoice := NewForkChoice(state, rpc)
	reputationManager := NewReputationManager(state, rpc)
	rewardCalculator := NewRewardCalculator(config, state, reputationManager)

	// Parse validator address
	validatorAddr := common.HexToAddress(config.ValidatorAddress)

	// Generate or load validator private key
	privKey := make([]byte, 32)
	rand.Read(privKey)

	ctx, cancel := context.WithCancel(context.Background())

	engine := &Engine{
		config:              config,
		state:               state,
		rpc:                 rpc,
		proposerSelector:    proposerSelector,
		attestationPool:     attestationPool,
		finalityEngine:      finalityEngine,
		forkChoice:          forkChoice,
		reputationManager:   reputationManager,
		rewardCalculator:    rewardCalculator,
		validatorAddress:    validatorAddr,
		validatorPrivateKey: privKey,
		ctx:                 ctx,
		cancel:              cancel,
		stats:               &Stats{StartTime: time.Now()},
		newSlotCh:           make(chan uint64, 10),
	}

	return engine, nil
}

// Start starts the beacon engine
func (e *Engine) Start() error {
	log.Info("🚀 EQUA Beacon Engine starting",
		"validator", e.validatorAddress.Hex(),
		"slotDuration", e.config.SlotDuration,
		"slotsPerEpoch", e.config.SlotsPerEpoch)

	// Load validators from execution layer
	if err := e.loadValidators(); err != nil {
		log.Warn("Failed to load validators", "error", err)
	}

	// Start slot ticker
	e.wg.Add(1)
	go e.slotTicker()

	// Start slot processor
	e.wg.Add(1)
	go e.slotProcessor()

	// Start attestation collector
	e.wg.Add(1)
	go e.attestationCollector()

	// Start finality checker
	e.wg.Add(1)
	go e.finalityChecker()

	// Start reputation updater
	e.wg.Add(1)
	go e.reputationUpdater()

	log.Info("✅ EQUA Beacon Engine started successfully")

	return nil
}

// Stop stops the beacon engine
func (e *Engine) Stop() {
	log.Info("🛑 Stopping EQUA Beacon Engine...")

	e.cancel()
	e.wg.Wait()

	log.Info("✅ EQUA Beacon Engine stopped")
}

// slotTicker generates slot ticks
func (e *Engine) slotTicker() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.SlotDuration)
	defer ticker.Stop()

	slot := e.state.Slot

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			slot++
			select {
			case e.newSlotCh <- slot:
			default:
				log.Warn("Slot channel full, skipping slot", "slot", slot)
			}
		}
	}
}

// slotProcessor processes slots
func (e *Engine) slotProcessor() {
	defer e.wg.Done()

	for {
		select {
		case <-e.ctx.Done():
			return
		case slot := <-e.newSlotCh:
			if err := e.processSlot(slot); err != nil {
				log.Error("Failed to process slot", "slot", slot, "error", err)
			}
		}
	}
}

// processSlot processes a single slot
func (e *Engine) processSlot(slot uint64) error {
	startTime := time.Now()

	e.mu.Lock()
	e.state.Slot = slot
	e.state.Epoch = slot / e.config.SlotsPerEpoch
	e.mu.Unlock()

	// Select proposer
	result, err := e.proposerSelector.SelectProposer(slot)
	if err != nil {
		return fmt.Errorf("failed to select proposer: %w", err)
	}

	log.Info("📍 Slot",
		"slot", slot,
		"epoch", e.state.Epoch,
		"proposer", result.Proposer.Hex()[:10]+"...")

	// Check if we are the proposer
	if result.Proposer == e.validatorAddress {
		if err := e.proposeBlock(slot); err != nil {
			log.Error("Failed to propose block", "error", err)
			e.stats.MissedSlots++
			return err
		}
		e.stats.BlocksProposed++
	}

	// Update stats
	e.stats.SlotsProcessed++
	slotTime := time.Since(startTime)
	e.stats.LastSlotTime = slotTime
	e.updateAverageSlotTime(slotTime)

	return nil
}

// proposeBlock proposes a block
func (e *Engine) proposeBlock(slot uint64) error {
	log.Info("🎯 Proposing block", "slot", slot, "validator", e.validatorAddress.Hex()[:10]+"...")

	// Get parent block
	parentHash := e.forkChoice.GetHead()
	if parentHash == (common.Hash{}) {
		// Genesis
		parentHash = common.Hash{}
	}

	// Generate random for PoW
	randomBytes := make([]byte, 32)
	rand.Read(randomBytes)
	random := common.BytesToHash(randomBytes)

	// Build payload attributes
	attrs := map[string]interface{}{
		"timestamp":             hexutil.Uint64(time.Now().Unix()),
		"prevRandao":            random,
		"suggestedFeeRecipient": e.validatorAddress,
	}

	// Forkchoice state
	fcs := map[string]interface{}{
		"headBlockHash":      parentHash,
		"safeBlockHash":      parentHash,
		"finalizedBlockHash": e.state.FinalizedHash,
	}

	// Call Engine API
	result, err := e.rpc.CallEngine("engine_forkchoiceUpdatedV2", []interface{}{fcs, attrs})
	if err != nil {
		return fmt.Errorf("forkchoiceUpdated failed: %w", err)
	}

	response := result.(map[string]interface{})
	payloadIDHex, ok := response["payloadId"].(string)
	if !ok {
		return errors.New("no payload ID returned")
	}

	// Wait for payload to be built
	time.Sleep(500 * time.Millisecond)

	// Get payload
	payload, err := e.rpc.CallEngine("engine_getPayloadV2", []interface{}{payloadIDHex})
	if err != nil {
		return fmt.Errorf("getPayload failed: %w", err)
	}

	executionPayload := payload.(map[string]interface{})["executionPayload"]

	// Submit payload
	_, err = e.rpc.CallEngine("engine_newPayloadV2", []interface{}{executionPayload})
	if err != nil {
		return fmt.Errorf("newPayload failed: %w", err)
	}

	// Get block hash
	blockData := executionPayload.(map[string]interface{})
	blockHashHex := blockData["blockHash"].(string)
	blockHash := common.HexToHash(blockHashHex)

	// Update forkchoice
	newFcs := map[string]interface{}{
		"headBlockHash":      blockHash,
		"safeBlockHash":      blockHash,
		"finalizedBlockHash": e.state.FinalizedHash,
	}
	_, err = e.rpc.CallEngine("engine_forkchoiceUpdatedV2", []interface{}{newFcs, nil})
	if err != nil {
		log.Warn("Failed to update forkchoice", "error", err)
	}

	// Get block number
	blockNumber, _ := e.rpc.GetBlockNumberByHash(blockHash)

	// Add to fork choice
	e.forkChoice.AddBlock(blockHash, blockNumber, parentHash)

	// Create attestation for our own block
	validator := e.state.Validators[e.validatorAddress]
	if validator != nil {
		att, err := e.attestationPool.CreateAttestation(slot, blockHash, validator, e.validatorPrivateKey)
		if err == nil {
			e.attestationPool.AddAttestation(att)
		}
	}

	log.Info("✨ Block proposed successfully",
		"slot", slot,
		"blockNumber", blockNumber,
		"blockHash", blockHash.Hex()[:10]+"...")

	return nil
}

// attestationCollector collects attestations
func (e *Engine) attestationCollector() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.SlotDuration / 3)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			// In production, this would listen for attestations from network
			// For now, we just track our own attestations
		}
	}
}

// finalityChecker checks for finality
func (e *Engine) finalityChecker() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.config.SlotDuration)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.checkFinality()
		}
	}
}

// checkFinality checks recent blocks for finality
func (e *Engine) checkFinality() {
	head := e.forkChoice.GetHead()
	if head == (common.Hash{}) {
		return
	}

	blockNumber, err := e.rpc.GetBlockNumberByHash(head)
	if err != nil {
		return
	}

	// Process block for finality
	e.finalityEngine.ProcessBlock(head, blockNumber, e.state.Slot)

	// Check if can finalize
	finalized, err := e.finalityEngine.CheckFinality(head, blockNumber)
	if err != nil {
		log.Debug("Finality check failed", "error", err)
	}

	if finalized {
		e.stats.LastFinalizedEpoch = e.state.Epoch
	}
}

// reputationUpdater updates validator reputations
func (e *Engine) reputationUpdater() {
	defer e.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.reputationManager.ApplyDecay()
		}
	}
}

// loadValidators loads validators from execution layer
func (e *Engine) loadValidators() error {
	validators, err := e.rpc.GetValidators()
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for i, v := range validators {
		if v.Reputation == nil {
			v.Reputation = e.reputationManager.GetReputation(v.Address)
		}

		e.state.Validators[v.Address] = v
		e.state.ValidatorIndices[v.Address] = uint64(i)

		if v.Active {
			e.state.TotalStake.Add(e.state.TotalStake, v.Stake)
			e.state.ActiveValidators++
		}
	}

	log.Info("📝 Validators loaded",
		"count", len(validators),
		"active", e.state.ActiveValidators,
		"totalStake", e.state.TotalStake)

	return nil
}

// GetStats returns engine statistics
func (e *Engine) GetStats() *Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := *e.stats
	stats.Uptime = time.Since(stats.StartTime)

	return &stats
}

func (e *Engine) updateAverageSlotTime(newTime time.Duration) {
	if e.stats.SlotsProcessed == 1 {
		e.stats.AverageSlotTime = newTime
		return
	}

	total := e.stats.AverageSlotTime * time.Duration(e.stats.SlotsProcessed-1)
	total += newTime
	e.stats.AverageSlotTime = total / time.Duration(e.stats.SlotsProcessed)
}
