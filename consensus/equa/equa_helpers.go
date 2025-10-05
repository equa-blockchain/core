// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"errors"
	"math/big"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/core/state"
	"github.com/equa/go-equa/core/tracing"
	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/crypto"
	"github.com/equa/go-equa/log"
	"github.com/holiman/uint256"
)

// selectProposer selects the next block proposer using hybrid PoS+PoW
// This is a deterministic algorithm - all validators calculate the same result
func (e *Equa) selectProposer(blockNumber uint64, parent *types.Header) (common.Address, error) {
	// Get top validators by stake
	topValidators := e.stakeManager.GetTopStakers(100)
	if len(topValidators) == 0 {
		// Dev mode fallback: Accept any proposer (will be set by miner config)
		return common.Address{}, nil
	}

	// Get total stake for normalization
	totalStake := big.NewInt(0)
	for _, validator := range topValidators {
		stake := e.stakeManager.GetStake(validator.Address)
		totalStake.Add(totalStake, stake)
	}

	if totalStake.Cmp(big.NewInt(0)) == 0 {
		// Fallback to first validator if no stake
		return topValidators[0].Address, nil
	}

	// ============================================
	// HYBRID PoS+PoW SELECTION ALGORITHM
	// ============================================
	// 1. Generate deterministic challenge from parent block
	//    This ensures all nodes calculate the same challenge
	var parentHash common.Hash
	if parent != nil {
		parentHash = parent.Hash()
	}
	challenge := e.powEngine.GenerateChallenge(parentHash, big.NewInt(int64(blockNumber)))

	// 2. Each validator competes with weighted score
	bestScore := big.NewInt(0)
	var selectedValidator common.Address

	for _, validator := range topValidators {
		// PoW Component: Hash(challenge + validator_address + blockNumber)
		// This adds entropy while remaining deterministic
		data := append(challenge.Bytes(), validator.Address.Bytes()...)
		data = append(data, big.NewInt(int64(blockNumber)).Bytes()...)
		hashValue := crypto.Keccak256Hash(data)

		// Convert hash to big.Int for calculation
		hashQuality := new(big.Int).SetBytes(hashValue.Bytes())

		// PoS Component: Stake weight (normalized to 10000 for precision)
		validatorStake := e.stakeManager.GetStake(validator.Address)
		stakeWeight := new(big.Int).Mul(validatorStake, big.NewInt(10000))
		stakeWeight.Div(stakeWeight, totalStake)

		// HYBRID SCORE = hashQuality * stakeWeight
		// Higher stake = higher probability, but PoW adds randomness
		score := new(big.Int).Mul(hashQuality, stakeWeight)

		if score.Cmp(bestScore) > 0 {
			bestScore = score
			selectedValidator = validator.Address
		}
	}

	if selectedValidator == (common.Address{}) {
		// Fallback: select by stake only (highest staker)
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
		burnAmountU256, _ := uint256.FromBig(burnAmount)
		state.AddBalance(burnAddress, burnAmountU256, tracing.BalanceChangeUnspecified)

		// Give MEV reward to proposer
		proposerMEVRewardU256, _ := uint256.FromBig(proposerMEVReward)
		state.AddBalance(header.Coinbase, proposerMEVRewardU256, tracing.BalanceIncreaseRewardMineBlock)

		// Emit MEV burn event
		log.Info("🔥 MEV burned",
			"amount", burnAmount.String(),
			"proposerReward", proposerMEVReward.String(),
			"proposer", header.Coinbase.Hex()[:10]+"...")
	}
}

// applyBlockRewards applies block rewards to the proposer
func (e *Equa) applyBlockRewards(header *types.Header, state *state.StateDB) {
	// Block reward: 2 EQUA per block
	blockReward := big.NewInt(int64(e.config.ValidatorReward))
	blockRewardU256, _ := uint256.FromBig(blockReward)
	state.AddBalance(header.Coinbase, blockRewardU256, tracing.BalanceIncreaseRewardMineBlock)

	// Update proposer's last block
	e.stakeManager.UpdateLastBlock(header.Coinbase, header.Number.Uint64())
}

// hasEncryptedTxs checks if there are encrypted transactions in the block
func (e *Equa) hasEncryptedTxs(txs []*types.Transaction) bool {
	// Check for encrypted transaction markers
	for _, tx := range txs {
		data := tx.Data()
		// Look for EQUA encryption markers
		if len(data) > 4 {
			// Check for EQUA encryption header: 0x45515541 ("EQUA")
			if data[0] == 0x45 && data[1] == 0x51 && data[2] == 0x55 && data[3] == 0x41 {
				return true
			}
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
	data := tx.Data()
	// Check for EQUA encryption header: 0x45515541 ("EQUA")
	return len(data) > 4 &&
		data[0] == 0x45 && data[1] == 0x51 &&
		data[2] == 0x55 && data[3] == 0x41
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
	// Skip validation if no validators (dev mode)
	validators := e.stakeManager.GetValidators()
	if len(validators) == 0 {
		return nil
	}

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
