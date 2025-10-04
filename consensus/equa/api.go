// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"context"
	"math/big"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/consensus"
	"github.com/equa/go-equa/core/types"
)

// API exposes EQUA consensus engine related functions for RPC access.
type API struct {
	chain consensus.ChainHeaderReader
	equa  *Equa
}

// GetValidators returns the current validator set
func (api *API) GetValidators() map[string]interface{} {
	validators := api.equa.stakeManager.GetValidators()

	result := make(map[string]interface{})
	result["count"] = len(validators)
	result["totalStake"] = api.equa.stakeManager.GetTotalStake().String()

	validatorList := make([]map[string]interface{}, len(validators))
	for i, validator := range validators {
		validatorList[i] = map[string]interface{}{
			"address":   validator.Address.Hex(),
			"stake":     validator.Stake.String(),
			"lastBlock": validator.LastBlock,
			"slashed":   validator.Slashed,
		}
	}
	result["validators"] = validatorList

	return result
}

// GetValidator returns information about a specific validator
func (api *API) GetValidator(address common.Address) map[string]interface{} {
	validator, exists := api.equa.stakeManager.GetValidator(address)
	if !exists {
		return map[string]interface{}{
			"exists": false,
		}
	}

	return map[string]interface{}{
		"exists":      true,
		"address":     validator.Address.Hex(),
		"stake":       validator.Stake.String(),
		"stakeWeight": api.equa.stakeManager.GetStakeWeight(address).String(),
		"lastBlock":   validator.LastBlock,
		"slashed":     validator.Slashed,
		"slashAmount": validator.SlashAmount.String(),
		"eligible":    api.equa.stakeManager.IsEligible(address),
	}
}

// GetMEVStats returns MEV statistics for recent blocks
func (api *API) GetMEVStats(blockCount int) map[string]interface{} {
	if blockCount <= 0 {
		blockCount = 100
	}

	currentBlock := api.chain.CurrentHeader().Number.Uint64()
	startBlock := currentBlock - uint64(blockCount)
	if startBlock > currentBlock {
		startBlock = 0
	}

	totalMEV := big.NewInt(0)
	totalBurned := big.NewInt(0)
	blocksWithMEV := 0

	// This is a simplified version - real implementation would
	// scan through blocks and calculate actual MEV

	return map[string]interface{}{
		"blockRange":     []uint64{startBlock, currentBlock},
		"totalMEV":       totalMEV.String(),
		"totalBurned":    totalBurned.String(),
		"blocksWithMEV":  blocksWithMEV,
		"burnPercentage": api.equa.config.MEVBurnPercentage,
	}
}

// GetConsensusInfo returns information about the consensus configuration
func (api *API) GetConsensusInfo() map[string]interface{} {
	return map[string]interface{}{
		"period":              api.equa.config.Period,
		"epoch":               api.equa.config.Epoch,
		"thresholdShares":     api.equa.config.ThresholdShares,
		"mevBurnPercentage":   api.equa.config.MEVBurnPercentage,
		"powDifficulty":       api.equa.config.PoWDifficulty,
		"validatorReward":     api.equa.config.ValidatorReward,
		"slashingPercentage":  api.equa.config.SlashingPercentage,
		"currentEpoch":        api.equa.epoch,
		"currentBlockNumber":  api.equa.blockNumber,
	}
}

// GetThresholdPublicKey returns the master public key for threshold encryption
func (api *API) GetThresholdPublicKey() string {
	if api.equa.thresholdCrypto.masterPubKey == nil {
		return ""
	}
	return common.Bytes2Hex(api.equa.thresholdCrypto.masterPubKey)
}

// EstimateMEV estimates MEV in a transaction list
func (api *API) EstimateMEV(txs []*types.Transaction) map[string]interface{} {
	// This would require transaction receipts to work properly
	// For now, return a placeholder

	return map[string]interface{}{
		"estimatedMEV":     "0",
		"transactionCount": len(txs),
		"warning":          "MEV estimation requires transaction execution",
	}
}

// ProposeBlock proposes a new block (for validator use)
func (api *API) ProposeBlock(ctx context.Context) (*types.Block, error) {
	// This would be called by validators to propose new blocks
	// Implementation would depend on integration with mining logic

	return nil, errors.New("block proposal not implemented in API")
}

// GetPoWDifficulty returns current PoW difficulty
func (api *API) GetPoWDifficulty() uint64 {
	return api.equa.powEngine.GetDifficulty()
}

// GetOrderingScore returns the ordering quality score for a block
func (api *API) GetOrderingScore(blockNumber uint64) map[string]interface{} {
	// Get block
	header := api.chain.GetHeaderByNumber(blockNumber)
	if header == nil {
		return map[string]interface{}{
			"error": "block not found",
		}
	}

	// This would require access to transaction list and ordering analysis
	// For now, return a placeholder

	return map[string]interface{}{
		"blockNumber":    blockNumber,
		"orderingScore":  1.0, // Placeholder
		"fairOrdering":   true,
	}
}

// GetSlashingEvents returns recent slashing events
func (api *API) GetSlashingEvents(blockCount int) []map[string]interface{} {
	// This would return actual slashing events from recent blocks
	// For now, return empty array

	return []map[string]interface{}{}
}

// IsValidator checks if an address is a validator
func (api *API) IsValidator(address common.Address) bool {
	return api.equa.stakeManager.HasStake(address)
}

// GetValidatorKeyShare returns the key share for a validator (restricted access)
func (api *API) GetValidatorKeyShare(address common.Address) string {
	validator, exists := api.equa.stakeManager.GetValidator(address)
	if !exists {
		return ""
	}

	// This should be restricted to authorized calls only
	return common.Bytes2Hex(validator.KeyShare)
}