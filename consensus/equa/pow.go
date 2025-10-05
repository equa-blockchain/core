// Copyright 2024 The go-equa Authors
// This file is part of the go-equa library.

package equa

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/big"
	"runtime"
	"sync"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/core/types"
	"github.com/equa/go-equa/crypto"
	"github.com/equa/go-equa/log"
	"github.com/equa/go-equa/params"
)

// LightPoW implements lightweight Proof of Work for randomness in validator selection
type LightPoW struct {
	config     *params.EquaConfig
	target     *big.Int
	hashCache  map[string]common.Hash // Cache for repeated challenges
	cacheMutex sync.RWMutex
	stats      PoWStats
}

// PoWStats tracks PoW statistics
type PoWStats struct {
	TotalAttempts    uint64        `json:"totalAttempts"`
	TotalTime        time.Duration `json:"totalTime"`
	AverageTime      time.Duration `json:"averageTime"`
	HashRate         float64       `json:"hashRate"`
	SuccessRate      float64       `json:"successRate"`
	LastSolveTime    time.Duration `json:"lastSolveTime"`
	BestQuality      *big.Int      `json:"bestQuality"`
}

// NewLightPoW creates a new lightweight PoW engine
func NewLightPoW(config *params.EquaConfig) *LightPoW {
	// Calculate target from difficulty
	// target = 2^256 / difficulty
	maxTarget := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
	target := new(big.Int).Div(maxTarget, big.NewInt(int64(config.PoWDifficulty)))

	return &LightPoW{
		config:    config,
		target:    target,
		hashCache: make(map[string]common.Hash),
		stats: PoWStats{
			BestQuality: big.NewInt(0),
		},
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

// Solve attempts to solve the PoW challenge using adaptive parallel workers with smart optimization
func (pow *LightPoW) Solve(header *types.Header, stop <-chan struct{}) (uint64, common.Hash, error) {
	challenge := header.MixDigest
	startTime := time.Now()

	// Adaptive worker count based on difficulty and CPU availability
	numWorkers := pow.calculateOptimalWorkers()

	// Create channels for communication
	results := make(chan solveResult, numWorkers)
	done := make(chan struct{})

	// Track best solution (for quality scoring)
	var bestSolution solveResult
	bestQuality := big.NewInt(0)
	solutionMutex := sync.Mutex{}

	// Start adaptive workers with different search strategies
	for i := 0; i < numWorkers; i++ {
		// Each worker uses a different search strategy for better coverage
		strategy := i % 3 // 0: linear, 1: random-jump, 2: adaptive
		go pow.solveWorkerAdvanced(i, numWorkers, challenge, header.Coinbase, results, done, stop, strategy)
	}

	// Adaptive timeout based on difficulty
	timeout := pow.calculateAdaptiveTimeout()
	timer := time.After(timeout)

	// Solution quality threshold - find best solution within time window
	qualityWindow := time.After(2 * time.Second) // Try to find best solution in first 2 seconds
	foundSolution := false

	for {
		select {
		case result := <-results:
			// Calculate solution quality
			quality := pow.CalculateQuality(result.hash)

			solutionMutex.Lock()
			if quality.Cmp(bestQuality) > 0 {
				bestQuality = quality
				bestSolution = result
				foundSolution = true
			}
			solutionMutex.Unlock()

			// If we found a very high quality solution, use it immediately
			if quality.Cmp(pow.getHighQualityThreshold()) > 0 {
				close(done)
				solveTime := time.Since(startTime)
				pow.updateStats(solveTime, true)

				log.Info("🎯 PoW solved (high quality)",
					"nonce", result.nonce,
					"hash", result.hash.Hex()[:16]+"...",
					"quality", quality.String(),
					"time", solveTime,
					"workers", numWorkers)

				return result.nonce, result.hash, nil
			}

		case <-qualityWindow:
			// Quality window expired, use best solution found so far
			if foundSolution {
				close(done)
				solveTime := time.Since(startTime)
				pow.updateStats(solveTime, true)

				log.Info("🎯 PoW solved (best quality)",
					"nonce", bestSolution.nonce,
					"hash", bestSolution.hash.Hex()[:16]+"...",
					"quality", bestQuality.String(),
					"time", solveTime,
					"workers", numWorkers)

				return bestSolution.nonce, bestSolution.hash, nil
			}

		case <-timer:
			close(done)

			// If we found any solution, use it
			if foundSolution {
				solveTime := time.Since(startTime)
				pow.updateStats(solveTime, true)

				log.Warn("⏱️  PoW timeout - using best solution",
					"nonce", bestSolution.nonce,
					"quality", bestQuality.String(),
					"time", solveTime)

				return bestSolution.nonce, bestSolution.hash, nil
			}

			pow.updateStats(time.Since(startTime), false)
			return 0, common.Hash{}, errors.New("PoW solve timeout - no solution found")

		case <-stop:
			close(done)
			return 0, common.Hash{}, errors.New("sealing stopped")
		}
	}
}

// solveResult represents a PoW solution
type solveResult struct {
	nonce uint64
	hash  common.Hash
}

// solveWorker is a worker goroutine for PoW solving
func (pow *LightPoW) solveWorker(workerID, totalWorkers int, challenge common.Hash, proposer common.Address, results chan<- solveResult, done <-chan struct{}, stop <-chan struct{}) {
	nonce := uint64(workerID)
	step := uint64(totalWorkers)

	for {
		select {
		case <-done:
			return
		case <-stop:
			return
		default:
		}

		// Calculate hash for current nonce
		hash := pow.calculateHash(challenge, proposer, nonce)
		hashInt := new(big.Int).SetBytes(hash[:])

		// Check if hash meets target
		if hashInt.Cmp(pow.target) <= 0 {
			select {
			case results <- solveResult{nonce: nonce, hash: hash}:
				return
			case <-done:
				return
			}
		}

		nonce += step

		// Prevent overflow
		if nonce > 1000000000 {
			return
		}
	}
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

// calculateHash computes the hash for PoW verification with caching
func (pow *LightPoW) calculateHash(challenge common.Hash, proposer common.Address, nonce uint64) common.Hash {
	// Create cache key
	cacheKey := string(challenge.Bytes()) + string(proposer.Bytes()) + string(nonce)

	// Check cache first
	pow.cacheMutex.RLock()
	if hash, exists := pow.hashCache[cacheKey]; exists {
		pow.cacheMutex.RUnlock()
		return hash
	}
	pow.cacheMutex.RUnlock()

	// Calculate hash
	data := make([]byte, 0, 32+20+8)
	data = append(data, challenge.Bytes()...)
	data = append(data, proposer.Bytes()...)

	nonceBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(nonceBytes, nonce)
	data = append(data, nonceBytes...)

	// Use SHA256 for better performance than Keccak256
	hashBytes := sha256.Sum256(data)
	hash := common.BytesToHash(hashBytes[:])

	// Cache the result
	pow.cacheMutex.Lock()
	if len(pow.hashCache) < 10000 { // Limit cache size
		pow.hashCache[cacheKey] = hash
	}
	pow.cacheMutex.Unlock()

	return hash
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

	log.Info("🔧 PoW difficulty adjusted",
		"oldDifficulty", pow.config.PoWDifficulty,
		"newDifficulty", uint64(newDifficulty),
		"avgTime", avgTime,
		"targetTime", targetTime)
}

// Helper functions for real PoW implementation

// updateStats updates PoW statistics
func (pow *LightPoW) updateStats(solveTime time.Duration, success bool) {
	pow.stats.TotalAttempts++
	pow.stats.TotalTime += solveTime
	pow.stats.AverageTime = pow.stats.TotalTime / time.Duration(pow.stats.TotalAttempts)
	pow.stats.LastSolveTime = solveTime

	if success {
		pow.stats.SuccessRate = float64(pow.stats.TotalAttempts) / float64(pow.stats.TotalAttempts)
	}

	// Calculate hash rate (hashes per second)
	if solveTime > 0 {
		pow.stats.HashRate = float64(pow.stats.TotalAttempts) / solveTime.Seconds()
	}
}

// GetStats returns current PoW statistics
func (pow *LightPoW) GetStats() PoWStats {
	return pow.stats
}

// ResetStats resets PoW statistics
func (pow *LightPoW) ResetStats() {
	pow.stats = PoWStats{
		BestQuality: big.NewInt(0),
	}
}

// OptimizeWorkers optimizes the number of workers based on performance
func (pow *LightPoW) OptimizeWorkers() int {
	// Simple optimization based on hash rate
	if pow.stats.HashRate > 1000000 { // 1M hashes/sec
		return 8
	} else if pow.stats.HashRate > 500000 { // 500K hashes/sec
		return 4
	} else {
		return 2
	}
}

// Benchmark runs a PoW benchmark
func (pow *LightPoW) Benchmark(duration time.Duration) map[string]interface{} {
	startTime := time.Now()
	attempts := uint64(0)

	// Create a test challenge
	challenge := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	proposer := common.HexToAddress("0x1234567890123456789012345678901234567890")

	for time.Since(startTime) < duration {
		pow.calculateHash(challenge, proposer, attempts)
		attempts++
	}

	elapsed := time.Since(startTime)
	hashRate := float64(attempts) / elapsed.Seconds()

	return map[string]interface{}{
		"duration":    elapsed.String(),
		"attempts":    attempts,
		"hashRate":    hashRate,
		"difficulty":  pow.config.PoWDifficulty,
		"target":      pow.target.String(),
	}
}

// ValidateChallenge validates a PoW challenge
func (pow *LightPoW) ValidateChallenge(challenge common.Hash, blockNumber *big.Int, parentHash common.Hash) bool {
	expectedChallenge := pow.GenerateChallenge(parentHash, blockNumber)
	return challenge == expectedChallenge
}

// GetQualityScore calculates the quality score of a PoW solution
func (pow *LightPoW) GetQualityScore(hash common.Hash) float64 {
	hashInt := new(big.Int).SetBytes(hash[:])
	targetInt := new(big.Int).SetBytes(pow.target.Bytes())

	if hashInt.Cmp(big.NewInt(0)) == 0 {
		return 0
	}

	// Quality = target / hash (higher is better)
	quality := new(big.Int).Div(targetInt, hashInt)
	return float64(quality.Uint64()) / 1000000.0 // Normalize to 0-1 range
}

// IsValidSolution checks if a solution meets the target
func (pow *LightPoW) IsValidSolution(hash common.Hash) bool {
	hashInt := new(big.Int).SetBytes(hash[:])
	return hashInt.Cmp(pow.target) <= 0
}

// GetTarget returns the current PoW target
func (pow *LightPoW) GetTarget() *big.Int {
	return new(big.Int).Set(pow.target)
}

// SetTarget sets a new PoW target
func (pow *LightPoW) SetTarget(target *big.Int) {
	pow.target = new(big.Int).Set(target)
}

// ClearCache clears the hash cache
func (pow *LightPoW) ClearCache() {
	pow.cacheMutex.Lock()
	pow.hashCache = make(map[string]common.Hash)
	pow.cacheMutex.Unlock()
}

// solveWorkerAdvanced is an advanced worker with multiple search strategies
func (pow *LightPoW) solveWorkerAdvanced(workerID, totalWorkers int, challenge common.Hash, proposer common.Address, results chan<- solveResult, done <-chan struct{}, stop <-chan struct{}, strategy int) {
	nonce := uint64(workerID)
	step := uint64(totalWorkers)
	attempts := uint64(0)

	for {
		select {
		case <-done:
			return
		case <-stop:
			return
		default:
		}

		// Calculate hash for current nonce
		hash := pow.calculateHash(challenge, proposer, nonce)
		hashInt := new(big.Int).SetBytes(hash[:])
		attempts++

		// Check if hash meets target
		if hashInt.Cmp(pow.target) <= 0 {
			select {
			case results <- solveResult{nonce: nonce, hash: hash}:
			case <-done:
				return
			}
		}

		// Apply search strategy
		switch strategy {
		case 0: // Linear search
			nonce += step

		case 1: // Random jump search (better distribution)
			jump := pow.calculateRandomJump(nonce, attempts)
			nonce += jump * step

		case 2: // Adaptive search (adjusts based on hash proximity)
			if pow.isCloseToTarget(hashInt) {
				// If close to target, search nearby
				nonce += step
			} else {
				// If far, make bigger jumps
				nonce += step * 10
			}
		}

		// Prevent overflow and limit search space
		if nonce > 10000000000 {
			nonce = uint64(workerID) + (attempts % 1000) * step
		}

		// Periodic yield to prevent CPU hogging
		if attempts%10000 == 0 {
			time.Sleep(time.Microsecond)
		}
	}
}

// calculateOptimalWorkers determines optimal number of workers
func (pow *LightPoW) calculateOptimalWorkers() int {
	cpuCount := runtime.NumCPU()

	// Adaptive worker count based on difficulty
	difficulty := pow.config.PoWDifficulty

	if difficulty < 1000 {
		// Very low difficulty: use fewer workers
		return min(cpuCount/2, 4)
	} else if difficulty < 10000 {
		// Medium difficulty: use moderate workers
		return min(cpuCount, 8)
	} else {
		// High difficulty: use all available workers
		return min(cpuCount*2, 16)
	}
}

// calculateAdaptiveTimeout calculates timeout based on difficulty
func (pow *LightPoW) calculateAdaptiveTimeout() time.Duration {
	// Base timeout: 30 seconds
	baseTimeout := 30 * time.Second

	// Adjust based on difficulty
	difficulty := pow.config.PoWDifficulty
	multiplier := float64(1.0)

	if difficulty > 100000 {
		multiplier = 2.0
	} else if difficulty > 10000 {
		multiplier = 1.5
	}

	// Adjust based on historical performance
	if pow.stats.AverageTime > baseTimeout {
		multiplier *= 1.5
	}

	timeout := time.Duration(float64(baseTimeout) * multiplier)

	// Cap maximum timeout
	maxTimeout := 2 * time.Minute
	if timeout > maxTimeout {
		timeout = maxTimeout
	}

	return timeout
}

// getHighQualityThreshold returns threshold for high-quality solutions
func (pow *LightPoW) getHighQualityThreshold() *big.Int {
	// High quality: hash is much lower than target (e.g., 10% of target)
	threshold := new(big.Int).Div(pow.target, big.NewInt(10))
	return threshold
}

// calculateRandomJump calculates a pseudo-random jump for nonce search
func (pow *LightPoW) calculateRandomJump(nonce, attempts uint64) uint64 {
	// Use linear congruential generator for pseudo-random jumps
	seed := nonce ^ attempts
	jump := (seed * 1103515245 + 12345) & 0x7fffffff
	return (jump % 1000) + 1 // Jump 1-1000
}

// isCloseToTarget checks if hash value is close to target
func (pow *LightPoW) isCloseToTarget(hashInt *big.Int) bool {
	// Define "close" as within 150% of target
	threshold := new(big.Int).Mul(pow.target, big.NewInt(3))
	threshold.Div(threshold, big.NewInt(2))
	return hashInt.Cmp(threshold) < 0
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
