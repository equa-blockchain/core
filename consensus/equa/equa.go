// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.
//
// The go-equa library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-equa library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-equa library. If not, see <http://www.gnu.org/licenses/>.

// Package equa implements the EQUA hybrid PoS+PoW anti-MEV consensus engine.
package equa

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/consensus"
	"github.com/equa/go-equa/core/state"
	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/core/vm"
	"github.com/equa/go-equa/ethdb"
	"github.com/equa/go-equa/log"
	"github.com/equa/go-equa/params"
	"github.com/equa/go-equa/rpc"
	"github.com/equa/go-equa/trie"
)

var (
	errUnknownBlock      = errors.New("unknown block")
	errInvalidPoW        = errors.New("invalid PoW solution")
	errInvalidValidator  = errors.New("invalid validator")
	errInsufficientStake = errors.New("insufficient stake")
	errMEVDetected       = errors.New("MEV extraction detected")
)

// Equa is the EQUA hybrid consensus engine that combines PoS with lightweight PoW for anti-MEV protection.
type Equa struct {
	config *params.EquaConfig // Consensus engine configuration parameters
	db     ethdb.Database      // Database to store and retrieve snapshot checkpoints

	// Core components
	stakeManager    *StakeManager    // Manages validator stakes and selection
	powEngine       *LightPoW       // Lightweight PoW for randomness
	mevDetector     *MEVDetector    // Detects and quantifies MEV extraction
	thresholdCrypto *ThresholdCrypto // Handles threshold encryption/decryption
	slasher         *Slasher        // Handles slashing for malicious behavior
	fairOrderer     *FairOrderer    // Implements fair transaction ordering

	// Runtime state
	currentValidators map[common.Address]*Validator // Current validator set
	blockNumber       uint64                        // Current block number
	epoch             uint64                        // Current epoch

	// Block production
	chain         consensus.ChainHeaderReader // Access to blockchain
	validatorAddr common.Address              // This node's validator address
	shouldSeal    func(common.Address) bool   // Check if we should seal next block
}

// New creates a new EQUA consensus engine.
func New(config *params.EquaConfig, chainConfig *params.ChainConfig, db ethdb.Database) *Equa {
	// Set default values if not specified
	if config.Period == 0 {
		config.Period = 12 // 12 seconds default
	}
	if config.Epoch == 0 {
		config.Epoch = 7200 // 24 hours default
	}
	if config.ThresholdShares == 0 {
		config.ThresholdShares = 2 // 2/3 default
	}
	if config.MEVBurnPercentage == 0 {
		config.MEVBurnPercentage = 80 // 80% burn default
	}

	equa := &Equa{
		config:            config,
		db:                db,
		currentValidators: make(map[common.Address]*Validator),
	}

	// Initialize components
	equa.stakeManager = NewStakeManager(db, config)
	equa.powEngine = NewLightPoW(config)
	equa.mevDetector = NewMEVDetector(config, chainConfig)
	equa.thresholdCrypto = NewThresholdCrypto(config)
	equa.slasher = NewSlasher(config, chainConfig)
	equa.fairOrderer = NewFairOrderer(config)

	// Initialize genesis validators if StakeManager is empty
	equa.initializeGenesisValidators()

	return equa
}

// initializeGenesisValidators initializes the default validators from genesis
func (e *Equa) initializeGenesisValidators() {
	// Check if already initialized
	if len(e.stakeManager.GetValidators()) > 0 {
		return
	}

	// Default 5 validators with 32 ETH stake each
	stake := new(big.Int).Mul(big.NewInt(32), big.NewInt(1e18))

	for i := 1; i <= 5; i++ {
		addr := common.HexToAddress(fmt.Sprintf("0x000000000000000000000000000000000000000%d", i))

		// Generate placeholder key shares (in production, these would be real BLS keys)
		keyShare := make([]byte, 32)
		pubKey := make([]byte, 48)
		copy(keyShare, addr.Bytes())
		copy(pubKey, addr.Bytes())

		err := e.stakeManager.AddValidator(addr, stake, keyShare, pubKey)
		if err != nil {
			log.Warn("Failed to add genesis validator", "address", addr.Hex(), "error", err)
			continue
		}

		log.Info("📝 Registered genesis validator", "address", addr.Hex(), "stake", stake.String())
	}

	totalStake := e.stakeManager.GetTotalStake()
	log.Info("🎉 Genesis validators initialized", "count", 5, "totalStake", totalStake.String())
}

// Author implements consensus.Engine, returning the Ethereum address recovered
// from the signature in the header's extra-data section.
func (e *Equa) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (e *Equa) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header) error {
	// Verify basic header fields
	if header.Number == nil {
		return errUnknownBlock
	}

	// Get parent header
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return errUnknownBlock
	}

	// Verify timestamp
	if header.Time <= parent.Time {
		return errors.New("invalid timestamp")
	}

	// Check if we have validators (production mode)
	validators := e.stakeManager.GetValidators()
	if len(validators) > 0 {
		// Production mode: Verify PoW solution
		if !e.powEngine.Verify(header, parent) {
			return errInvalidPoW
		}

		// Verify proposer has stake
		if !e.stakeManager.HasStake(header.Coinbase) {
			return errInvalidValidator
		}
	}
	// Dev mode: Skip PoW and stake verification
	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications.
func (e *Equa) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for _, header := range headers {
			err := e.VerifyHeader(chain, header)

			select {
			case <-abort:
				return
			case results <- err:
				if err != nil {
					log.Warn("Header verification failed", "number", header.Number, "err", err)
				}
			}
		}
	}()

	return abort, results
}

// VerifyUncles implements consensus.Engine, always returning an error for any
// uncles as this consensus mechanism doesn't permit uncles.
func (e *Equa) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}
	return nil
}

// Prepare implements consensus.Engine, preparing all the consensus fields of the
// header for running the transactions on top.
func (e *Equa) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	// Get parent header
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return errUnknownBlock
	}

	// Set basic header fields
	header.Time = uint64(time.Now().Unix())

	// Update block number and epoch
	e.blockNumber = header.Number.Uint64()
	e.epoch = e.blockNumber / e.config.Epoch

	// Check if we have validators (production mode with PoS+PoW)
	validators := e.stakeManager.GetValidators()
	if len(validators) > 0 {
		// Production mode: Use hybrid PoS+PoW
		header.Difficulty = big.NewInt(int64(e.config.PoWDifficulty))

		// Select proposer using hybrid PoS+PoW
		proposer, err := e.selectProposer(header.Number.Uint64(), parent)
		if err != nil {
			return err
		}
		header.Coinbase = proposer

		// Log proposer selection
		if e.validatorAddr != (common.Address{}) {
			if proposer == e.validatorAddr {
				log.Info("🎲 SELECTED as block proposer!",
					"block", header.Number,
					"validator", proposer.Hex()[:10]+"...")
			} else {
				log.Debug("Not our turn",
					"block", header.Number,
					"proposer", proposer.Hex()[:10]+"...",
					"us", e.validatorAddr.Hex()[:10]+"...")
			}
		}

		// Generate PoW challenge
		challenge := e.powEngine.GenerateChallenge(header.ParentHash, header.Number)
		header.MixDigest = challenge
	} else {
		// Dev mode: Skip PoW, use standard post-merge fields
		header.Difficulty = big.NewInt(0)
		// Coinbase is already set by miner config in dev mode
		// MixDigest (prevrandao) is set by Engine API in post-merge
	}

	return nil
}

// Finalize implements consensus.Engine, accumulating the block rewards.
func (e *Equa) Finalize(chain consensus.ChainHeaderReader, header *types.Header, statedb vm.StateDB, body *types.Body) {
	// Note: We don't apply rewards here because FinalizeAndAssemble handles that
	// Applying rewards in both places causes double application and hash mismatch
	// in Engine API flow (post-merge)
}

// FinalizeAndAssemble implements consensus.Engine, accumulating the block rewards,
// setting the final state and assembling the block.
func (e *Equa) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, body *types.Body, receipts []*types.Receipt) (*types.Block, error) {
	txs := body.Transactions
	uncles := body.Uncles

	// Decrypt transactions if they are encrypted
	if e.hasEncryptedTxs(txs) {
		decryptedTxs, err := e.decryptTransactions(txs)
		if err != nil {
			return nil, err
		}
		txs = decryptedTxs
	}

	// Apply fair ordering
	orderedTxs := e.fairOrderer.OrderTransactions(txs)

	// Process MEV detection and burning (we have receipts here)
	e.processMEVAndRewards(header, state, orderedTxs, receipts)

	// NOTE: In post-merge (Engine API) mode, block rewards are NOT applied here
	// The consensus layer (beacon) handles rewards via fee recipient in PayloadAttributes
	// Only apply rewards in legacy PoW mode (when difficulty > 0)
	if header.Difficulty.Cmp(big.NewInt(0)) > 0 {
		e.applyBlockRewards(header, state)
	}

	// Finalize state and update header root
	oldRoot := header.Root
	header.Root = state.IntermediateRoot(true)

	// Assemble and return the final block
	finalBody := &types.Body{
		Transactions: orderedTxs,
		Uncles:       uncles,
		Withdrawals:  body.Withdrawals,
	}

	finalBlock := types.NewBlock(header, finalBody, receipts, trie.NewStackTrie(nil))

	log.Info("📦 Assembled new block",
		"number", header.Number,
		"hash", finalBlock.Hash().Hex()[:10],
		"oldRoot", oldRoot.Hex()[:10],
		"newRoot", header.Root.Hex()[:10],
		"rootChanged", oldRoot != header.Root,
		"txs", len(orderedTxs),
		"coinbase", header.Coinbase.Hex()[:10],
	)

	return finalBlock, nil
}

// Seal implements consensus.Engine, attempting to create a sealed block using
// the local signing credentials.
func (e *Equa) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := types.CopyHeader(block.Header())

	// Check if we're running with validators (production PoS mode)
	validators := e.stakeManager.GetValidators()

	// In post-merge mode with beacons, the block is already sealed
	// Only do PoW in standalone mode without beacons
	if len(validators) > 0 && header.Difficulty.Cmp(big.NewInt(0)) == 0 {
		// Post-merge mode: block is already sealed by Engine API
		// Just return the block as-is
		log.Info("🔒 Sealed new block (post-merge)",
			"number", header.Number,
			"hash", block.Hash().Hex()[:10],
			"txs", len(block.Transactions()),
			"gasUsed", header.GasUsed,
			"miner", header.Coinbase.Hex()[:10],
		)

		select {
		case results <- block:
		default:
			log.Warn("Sealing result is not read by miner", "sealhash", e.SealHash(header))
		}

		return nil
	}

	// Legacy PoW mode (for testing/dev without beacons)
	nonce, mixDigest, err := e.powEngine.Solve(header, stop)
	if err != nil {
		return err
	}

	// Update header with PoW solution
	header.Nonce = types.EncodeNonce(nonce)
	header.MixDigest = mixDigest

	sealedBlock := block.WithSeal(header)

	// Log successful block seal
	log.Info("🔒 Sealed new block (PoW)",
		"number", header.Number,
		"hash", header.Hash().Hex()[:10],
		"txs", len(block.Transactions()),
		"gasUsed", header.GasUsed,
		"miner", header.Coinbase.Hex()[:10],
	)

	// Send the sealed block
	select {
	case results <- sealedBlock:
	default:
		log.Warn("Sealing result is not read by miner", "sealhash", e.SealHash(header))
	}

	return nil
}

// SealHash returns the hash of a block prior to it being sealed.
func (e *Equa) SealHash(header *types.Header) common.Hash {
	// In post-merge EQUA, we use the standard header hash
	// which includes all fields (BaseFee, WithdrawalsHash, BlobGasUsed, etc)
	return header.Hash()
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have based on the previous blocks in the chain and the
// current time.
func (e *Equa) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	// EQUA uses fixed lightweight PoW difficulty
	return big.NewInt(int64(e.config.PoWDifficulty))
}

// APIs implements consensus.Engine, returning the user facing RPC API.
func (e *Equa) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return []rpc.API{{
		Namespace: "equa",
		Version:   "1.0",
		Service:   NewAPI(chain, e),
		Public:    true,
	}}
}

// Close implements consensus.Engine. It's a noop for EQUA as there are no background threads.
func (e *Equa) Close() error {
	return nil
}

// Start begins the block production process (called by miner)
func (e *Equa) Start(chain consensus.ChainHeaderReader, currentBlock func() *types.Header, hasBadBlock func(common.Hash) bool) error {
	e.chain = chain

	validators := e.stakeManager.GetValidators()
	if len(validators) > 0 {
		log.Info("🚀 EQUA consensus engine started", "validators", len(validators), "period", e.config.Period)

		// Start block production loop
		go e.blockProductionLoop(chain, currentBlock)
	} else {
		log.Warn("⚠️  No validators registered, running in dev mode")
	}

	return nil
}

// blockProductionLoop continuously produces blocks when it's our turn
func (e *Equa) blockProductionLoop(chain consensus.ChainHeaderReader, currentBlock func() *types.Header) {
	ticker := time.NewTicker(time.Duration(e.config.Period) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			current := currentBlock()
			if current == nil {
				continue
			}

			nextBlockNumber := current.Number.Uint64() + 1

			// Check if it's our turn to propose
			proposer, err := e.selectProposer(nextBlockNumber, current)
			if err != nil {
				log.Debug("Failed to select proposer", "err", err)
				continue
			}

			// Check if we are the selected proposer
			if proposer == e.validatorAddr {
				log.Info("🎲 SELECTED as block proposer!",
					"block", nextBlockNumber,
					"validator", proposer.Hex()[:10]+"...")

				// Signal to miner to produce block
				// This will trigger the miner to call Prepare() and Seal()
				e.triggerBlockProduction()
			} else {
				log.Debug("Not our turn",
					"block", nextBlockNumber,
					"proposer", proposer.Hex()[:10]+"...",
					"us", e.validatorAddr.Hex()[:10]+"...")
			}
		}
	}
}

// triggerBlockProduction signals that we should produce a block
func (e *Equa) triggerBlockProduction() {
	// In EQUA, the miner package will automatically call Prepare() and Seal()
	// when it's time to produce a block. Our job is just to ensure the proposer
	// selection is correct, which happens in Prepare().
	log.Info("📦 Block production triggered - proposer selection complete")
}

// SetValidator sets this node's validator address (called during node init)
func (e *Equa) SetValidator(addr common.Address) {
	e.validatorAddr = addr
	log.Info("📝 Validator address configured", "address", addr.Hex())
}
