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
	"math/big"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/consensus"
	"github.com/equa/go-equa/core/state"
	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/ethdb"
	"github.com/equa/go-equa/log"
	"github.com/equa/go-equa/params"
	"github.com/equa/go-equa/rpc"
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
}

// New creates a new EQUA consensus engine.
func New(config *params.EquaConfig, db ethdb.Database) *Equa {
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
	equa.mevDetector = NewMEVDetector(config)
	equa.thresholdCrypto = NewThresholdCrypto(config)
	equa.slasher = NewSlasher(config)
	equa.fairOrderer = NewFairOrderer(config)

	return equa
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

	// Verify PoW solution
	if !e.powEngine.Verify(header, parent) {
		return errInvalidPoW
	}

	// Verify proposer has stake
	if !e.stakeManager.HasStake(header.Coinbase) {
		return errInvalidValidator
	}

	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications.
func (e *Equa) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
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
func (e *Equa) VerifyUncles(chain consensus.ChainHeaderReader, block *types.Block) error {
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
	header.Difficulty = big.NewInt(int64(e.config.PoWDifficulty))

	// Update block number and epoch
	e.blockNumber = header.Number.Uint64()
	e.epoch = e.blockNumber / e.config.Epoch

	// Select proposer using hybrid PoS+PoW
	proposer, err := e.selectProposer(header.Number.Uint64(), parent)
	if err != nil {
		return err
	}
	header.Coinbase = proposer

	// Generate PoW challenge
	challenge := e.powEngine.GenerateChallenge(header.ParentHash, header.Number)
	header.MixDigest = challenge

	return nil
}

// Finalize implements consensus.Engine, accumulating the block rewards,
// setting the final state and assembling the block.
func (e *Equa) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) {
	// Process MEV detection and burning
	e.processMEVAndRewards(header, state, txs, receipts)

	// Apply block rewards
	e.applyBlockRewards(header, state)
}

// FinalizeAndAssemble implements consensus.Engine, accumulating the block rewards,
// setting the final state and assembling the block.
func (e *Equa) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
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

	// Finalize the block
	e.Finalize(chain, header, state, orderedTxs, uncles, receipts)

	// Assemble and return the final block
	return types.NewBlock(header, orderedTxs, uncles, receipts, nil), nil
}

// Seal implements consensus.Engine, attempting to create a sealed block using
// the local signing credentials.
func (e *Equa) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := types.CopyHeader(block.Header())

	// Solve lightweight PoW
	nonce, mixDigest, err := e.powEngine.Solve(header, stop)
	if err != nil {
		return err
	}

	// Update header with PoW solution
	header.Nonce = types.EncodeNonce(nonce)
	header.MixDigest = mixDigest

	// Send the sealed block
	select {
	case results <- block.WithSeal(header):
	default:
		log.Warn("Sealing result is not read by miner", "sealhash", types.SealHash(header))
	}

	return nil
}

// SealHash returns the hash of a block prior to it being sealed.
func (e *Equa) SealHash(header *types.Header) common.Hash {
	return types.SealHash(header)
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
		Service:   &API{equa: e, chain: chain},
		Public:    true,
	}}
}

// Close implements consensus.Engine. It's a noop for EQUA as there are no background threads.
func (e *Equa) Close() error {
	return nil
}