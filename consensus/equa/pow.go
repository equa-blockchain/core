// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/crypto"
	"github.com/equa/go-equa/params"
)

// LightPoW implements lightweight Proof of Work for randomness in validator selection
type LightPoW struct {
	config *params.EquaConfig
	target *big.Int
}

// NewLightPoW creates a new lightweight PoW engine
func NewLightPoW(config *params.EquaConfig) *LightPoW {
	// Calculate target from difficulty
	// target = 2^256 / difficulty
	maxTarget := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
	target := new(big.Int).Div(maxTarget, big.NewInt(int64(config.PoWDifficulty)))

	return &LightPoW{
		config: config,
		target: target,
	}
}

// GenerateChallenge generates a PoW challenge for the given block
func (pow *LightPoW) GenerateChallenge(parentHash common.Hash, blockNumber *big.Int) common.Hash {
	// Challenge = hash(parentHash || blockNumber || timestamp || salt)
	data := make([]byte, 0, 32+8+8+8)
	data = append(data, parentHash.Bytes()...)

	// Add block number
	blockNumBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(blockNumBytes, blockNumber.Uint64())
	data = append(data, blockNumBytes...)

	// Add timestamp (epoch time)
	epochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(epochBytes, uint64(time.Now().Unix()))
	data = append(data, epochBytes...)

	// Add salt based on epoch
	epoch := blockNumber.Uint64() / pow.config.Epoch
	saltBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(saltBytes, epoch)
	data = append(data, saltBytes...)

	return crypto.Keccak256Hash(data)
}

// Solve attempts to solve the PoW challenge
func (pow *LightPoW) Solve(header *types.Header, stop <-chan struct{}) (uint64, common.Hash, error) {
	challenge := header.MixDigest
	nonce := uint64(0)
	maxIterations := uint64(1000000) // Limit iterations to prevent DoS

	for nonce < maxIterations {
		select {
		case <-stop:
			return 0, common.Hash{}, errors.New("sealing stopped")
		default:
		}

		// Calculate hash for current nonce
		hash := pow.calculateHash(challenge, header.Coinbase, nonce)
		hashInt := new(big.Int).SetBytes(hash[:])

		// Check if hash meets target
		if hashInt.Cmp(pow.target) <= 0 {
			return nonce, hash, nil
		}

		nonce++
	}

	return 0, common.Hash{}, errors.New("failed to find valid nonce")
}

// Verify checks if a PoW solution is valid
func (pow *LightPoW) Verify(header *types.Header, parent *types.Header) bool {
	// Verify the challenge was generated correctly
	expectedChallenge := pow.GenerateChallenge(parent.Hash(), header.Number)
	if header.MixDigest != expectedChallenge {
		return false
	}

	// Verify the nonce produces a valid hash
	nonce := header.Nonce.Uint64()
	hash := pow.calculateHash(header.MixDigest, header.Coinbase, nonce)
	hashInt := new(big.Int).SetBytes(hash[:])

	return hashInt.Cmp(pow.target) <= 0
}

// calculateHash computes the hash for PoW verification
func (pow *LightPoW) calculateHash(challenge common.Hash, proposer common.Address, nonce uint64) common.Hash {
	// Hash = Keccak256(challenge || proposer || nonce)
	data := make([]byte, 0, 32+20+8)
	data = append(data, challenge.Bytes()...)
	data = append(data, proposer.Bytes()...)

	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, nonce)
	data = append(data, nonceBytes...)

	return crypto.Keccak256Hash(data)
}

// GetDifficulty returns current PoW difficulty
func (pow *LightPoW) GetDifficulty() uint64 {
	return pow.config.PoWDifficulty
}

// SetDifficulty updates the PoW difficulty
func (pow *LightPoW) SetDifficulty(difficulty uint64) {
	pow.config.PoWDifficulty = difficulty

	// Recalculate target
	maxTarget := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
	pow.target = new(big.Int).Div(maxTarget, big.NewInt(int64(difficulty)))
}

// EstimateSolveTime estimates time to solve PoW based on current difficulty
func (pow *LightPoW) EstimateSolveTime() time.Duration {
	// Rough estimate: difficulty / (hash_rate)
	// Assuming ~1M hashes per second on average CPU
	hashRate := float64(1000000) // 1M hashes/sec
	expectedHashes := float64(pow.config.PoWDifficulty) / 2

	seconds := expectedHashes / hashRate
	return time.Duration(seconds * float64(time.Second))
}

// CalculateQuality calculates the quality of a PoW solution
// Better quality = lower hash value
func (pow *LightPoW) CalculateQuality(hash common.Hash) *big.Int {
	hashInt := new(big.Int).SetBytes(hash[:])

	// Quality = target / hash (higher is better)
	if hashInt.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0)
	}

	quality := new(big.Int).Div(pow.target, hashInt)
	return quality
}

// AdjustDifficulty adjusts difficulty based on recent block times
func (pow *LightPoW) AdjustDifficulty(recentBlocks []*types.Header, targetTime time.Duration) {
	if len(recentBlocks) < 2 {
		return
	}

	// Calculate average time between blocks
	totalTime := int64(0)
	for i := 1; i < len(recentBlocks); i++ {
		timeDiff := int64(recentBlocks[i].Time - recentBlocks[i-1].Time)
		totalTime += timeDiff
	}

	avgTime := time.Duration(totalTime / int64(len(recentBlocks)-1)) * time.Second
	targetTimeSeconds := targetTime.Seconds()
	avgTimeSeconds := avgTime.Seconds()

	// Adjust difficulty: if blocks too fast, increase difficulty
	// if blocks too slow, decrease difficulty
	adjustment := targetTimeSeconds / avgTimeSeconds

	newDifficulty := float64(pow.config.PoWDifficulty) * adjustment

	// Limit adjustment to prevent wild swings
	if adjustment > 1.5 {
		newDifficulty = float64(pow.config.PoWDifficulty) * 1.5
	} else if adjustment < 0.5 {
		newDifficulty = float64(pow.config.PoWDifficulty) * 0.5
	}

	// Ensure minimum difficulty
	if newDifficulty < 1000 {
		newDifficulty = 1000
	}

	pow.SetDifficulty(uint64(newDifficulty))
}