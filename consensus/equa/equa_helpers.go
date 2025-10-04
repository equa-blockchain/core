// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"errors"
	"math/big"
	"math/rand"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/core/state"
	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/crypto"
)

// selectProposer selects the next block proposer using hybrid PoS+PoW
func (e *Equa) selectProposer(blockNumber uint64, parent *types.Header) (common.Address, error) {
	// Get top validators by stake
	topValidators := e.stakeManager.GetTopStakers(100)
	if len(topValidators) == 0 {
		return common.Address{}, errors.New("no validators available")
	}

	// For genesis block, select randomly
	if blockNumber == 1 {
		rand.Seed(time.Now().UnixNano())
		return topValidators[rand.Intn(len(topValidators))].Address, nil
	}

	// Use PoW to add randomness to validator selection
	challenge := e.powEngine.GenerateChallenge(parent.Hash(), big.NewInt(int64(blockNumber)))

	// Each validator competes with weighted PoW
	bestScore := big.NewInt(0)
	var selectedValidator common.Address

	for _, validator := range topValidators {
		// Simple PoW competition: hash(challenge + validator_address)
		data := append(challenge.Bytes(), validator.Address.Bytes()...)
		hash := crypto.Keccak256Hash(data)

		// Calculate weighted score: hash_quality * stake_weight
		stakeWeight := e.stakeManager.GetStakeWeight(validator.Address)
		hashQuality := e.powEngine.CalculateQuality(hash)

		score := new(big.Int).Mul(hashQuality, stakeWeight)

		if score.Cmp(bestScore) > 0 {
			bestScore = score
			selectedValidator = validator.Address
		}
	}

	if selectedValidator == (common.Address{}) {
		// Fallback: select by stake only
		return topValidators[0].Address, nil
	}

	return selectedValidator, nil
}

// processMEVAndRewards handles MEV detection and reward distribution
func (e *Equa) processMEVAndRewards(header *types.Header, state *state.StateDB, txs []*types.Transaction, receipts []*types.Receipt) {
	// Detect MEV in the block
	totalMEV := e.mevDetector.DetectMEV(txs, receipts)

	if totalMEV.Cmp(big.NewInt(0)) > 0 {
		// Calculate burn amount (80% of MEV)
		burnAmount := new(big.Int).Mul(totalMEV, big.NewInt(int64(e.config.MEVBurnPercentage)))
		burnAmount.Div(burnAmount, big.NewInt(100))

		// Calculate proposer reward (20% of MEV)
		proposerMEVReward := new(big.Int).Sub(totalMEV, burnAmount)

		// Burn MEV: send to zero address
		burnAddress := common.Address{}
		state.AddBalance(burnAddress, burnAmount)

		// Give MEV reward to proposer
		state.AddBalance(header.Coinbase, proposerMEVReward)

		// Emit MEV burn event
		// TODO: Add event emission
	}
}

// applyBlockRewards applies block rewards to the proposer
func (e *Equa) applyBlockRewards(header *types.Header, state *state.StateDB) {
	// Block reward: 2 EQUA per block
	blockReward := big.NewInt(int64(e.config.ValidatorReward))
	state.AddBalance(header.Coinbase, blockReward)

	// Update proposer's last block
	e.stakeManager.UpdateLastBlock(header.Coinbase, header.Number.Uint64())
}

// hasEncryptedTxs checks if there are encrypted transactions in the block
func (e *Equa) hasEncryptedTxs(txs []*types.Transaction) bool {
	// Simple check: look for transactions with special flag or format
	// In a full implementation, this would check for EncryptedTransaction type
	for _, tx := range txs {
		// Placeholder: check if transaction data starts with encryption marker
		data := tx.Data()
		if len(data) > 4 && string(data[:4]) == "ENCR" {
			return true
		}
	}
	return false
}

// decryptTransactions decrypts encrypted transactions using threshold cryptography
func (e *Equa) decryptTransactions(txs []*types.Transaction) ([]*types.Transaction, error) {
	decryptedTxs := make([]*types.Transaction, 0, len(txs))

	for _, tx := range txs {
		if e.isEncryptedTx(tx) {
			// Get validator key shares
			validators := e.stakeManager.GetValidators()
			if len(validators) < int(e.config.ThresholdShares) {
				return nil, errors.New("insufficient validators for threshold decryption")
			}

			// Collect key shares
			keyShares := make([][]byte, 0, len(validators))
			validatorAddrs := make([]common.Address, 0, len(validators))

			for _, validator := range validators {
				keyShares = append(keyShares, validator.KeyShare)
				validatorAddrs = append(validatorAddrs, validator.Address)

				if len(keyShares) >= int(e.config.ThresholdShares) {
					break
				}
			}

			// Decrypt transaction
			decryptedTx, err := e.thresholdCrypto.DecryptTransaction(tx, keyShares)
			if err != nil {
				// Skip invalid encrypted transactions
				continue
			}

			decryptedTxs = append(decryptedTxs, decryptedTx)
		} else {
			// Keep non-encrypted transactions as-is
			decryptedTxs = append(decryptedTxs, tx)
		}
	}

	return decryptedTxs, nil
}

// isEncryptedTx checks if a transaction is encrypted
func (e *Equa) isEncryptedTx(tx *types.Transaction) bool {
	// Simple check for encrypted transaction marker
	data := tx.Data()
	return len(data) > 4 && string(data[:4]) == "ENCR"
}

// checkSlashingConditions checks for slashing conditions and applies penalties
func (e *Equa) checkSlashingConditions(header *types.Header, txs []*types.Transaction, receipts []*types.Receipt) error {
	proposer := header.Coinbase

	// Check for MEV extraction by proposer
	if e.slasher.DetectMEVExtraction(proposer, txs, receipts) {
		err := e.stakeManager.SlashValidator(proposer, e.config.SlashingPercentage, "MEV extraction")
		if err != nil {
			return err
		}
	}

	// Check for transaction reordering
	if e.slasher.DetectTxReordering(txs) {
		err := e.stakeManager.SlashValidator(proposer, 10, "Transaction reordering")
		if err != nil {
			return err
		}
	}

	// Check for censorship
	if e.slasher.DetectCensorship(txs) {
		err := e.stakeManager.SlashValidator(proposer, 20, "Transaction censorship")
		if err != nil {
			return err
		}
	}

	return nil
}

// validateProposer checks if the block proposer is valid
func (e *Equa) validateProposer(header *types.Header, parent *types.Header) error {
	// Check if proposer has sufficient stake
	if !e.stakeManager.IsEligible(header.Coinbase) {
		return errInsufficientStake
	}

	// Verify proposer was correctly selected
	expectedProposer, err := e.selectProposer(header.Number.Uint64(), parent)
	if err != nil {
		return err
	}

	if header.Coinbase != expectedProposer {
		return errInvalidValidator
	}

	return nil
}