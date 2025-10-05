// Copyright 2024 The go-equa Authors
// EQUA Beacon Engine - Main Entry Point

package main

import (
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/equa/go-equa/cmd/equa-beacon-engine/engine"
	"github.com/equa/go-equa/log"
)

var (
	// Network
	executionEndpoint = flag.String("execution-endpoint", "http://localhost:8551", "Execution layer Engine API endpoint")
	rpcEndpoint       = flag.String("rpc-endpoint", "http://localhost:8545", "Execution layer JSON-RPC endpoint")
	jwtSecret         = flag.String("jwt-secret", "", "Path to JWT secret file for Engine API")

	// Validator
	validatorAddress = flag.String("validator-address", "", "Validator address (required)")
	validatorID      = flag.Int("validator-id", 0, "Validator ID for default address generation (1-5)")

	// Timing
	slotDuration  = flag.Duration("slot-duration", 0, "Slot duration (0 = auto-detect from genesis)")
	slotsPerEpoch = flag.Uint64("slots-per-epoch", 32, "Number of slots per epoch")

	// Performance
	minValidators = flag.Uint64("min-validators", 1, "Minimum active validators")
	maxValidators = flag.Uint64("max-validators", 100, "Maximum validators")

	// Advanced
	powInfluence  = flag.Float64("pow-influence", 0.3, "PoW influence in proposer selection (0-1)")
	mevBonus      = flag.Float64("mev-bonus", 0.2, "Reward bonus for no-MEV blocks")
	orderingBonus = flag.Float64("ordering-bonus", 0.15, "Reward bonus for fair ordering")
	minReputation = flag.Float64("min-reputation", 70.0, "Minimum reputation score to propose")
)

func main() {
	flag.Parse()

	// Setup logging
	glogger := log.NewGlogHandler(log.NewTerminalHandler(os.Stderr, true))
	glogger.Verbosity(log.LvlInfo)
	log.SetDefault(log.NewLogger(glogger))

	log.Info("🔷 EQUA Beacon Engine")
	log.Info("====================")

	// Validate flags
	if *validatorAddress == "" && *validatorID == 0 {
		log.Crit("Either --validator-address or --validator-id must be specified")
	}

	// Generate validator address if using ID
	validatorAddr := *validatorAddress
	if validatorAddr == "" && *validatorID > 0 {
		validatorAddr = fmt.Sprintf("0x000000000000000000000000000000000000000%d", *validatorID)
		log.Info("📝 Using default validator address", "id", *validatorID, "address", validatorAddr)
	}

	// Read and prepare JWT secret
	jwtSecretValue := readJWTSecret(*jwtSecret)
	if jwtSecretValue != "" {
		log.Info("✅ JWT secret loaded", "length", len(jwtSecretValue))
	}

	// Create configuration
	config := &engine.Config{
		// Network
		NetworkID:         3782,
		ChainID:           3782,
		ExecutionEndpoint: *executionEndpoint,
		RPCEndpoint:       *rpcEndpoint,
		JWTSecretPath:     jwtSecretValue,

		// Timing
		SlotDuration:  *slotDuration,
		SlotsPerEpoch: *slotsPerEpoch,

		// Consensus
		MinValidators: *minValidators,
		MaxValidators: *maxValidators,
		MinStake:      new(big.Int).Mul(big.NewInt(32), big.NewInt(1e18)), // 32 EQUA

		// Finality
		FinalityThreshold:  0.67, // 2/3
		JustificationDelay: 1,    // 1 slot
		FinalizationDelay:  2,    // 2 slots

		// Rewards
		BaseRewardPerEpoch:      new(big.Int).Mul(big.NewInt(2), big.NewInt(1e18)), // 2 EQUA per epoch
		MEVBonusMultiplier:      *mevBonus,
		OrderingBonusMultiplier: *orderingBonus,

		// Slashing
		SlashingPenalty:   0.5,  // 50%
		InactivityPenalty: 0.01, // 1%

		// PoW Integration
		PoWInfluence:  *powInfluence,
		MinPoWQuality: 1000,

		// Reputation
		ReputationDecayRate: 0.01,
		MinReputationScore:  *minReputation,

		// Validator
		ValidatorAddress: validatorAddr,
	}

	// Auto-detect slot duration if not set
	if config.SlotDuration == 0 {
		log.Info("⏱️  Auto-detecting slot duration from genesis...")
		period := detectBlockPeriod(config)
		config.SlotDuration = time.Duration(period) * time.Second
		log.Info("✅ Slot duration detected", "duration", config.SlotDuration)
	}

	// Create engine
	eng, err := engine.NewEngine(config)
	if err != nil {
		log.Crit("Failed to create engine", "error", err)
	}

	// Start engine
	if err := eng.Start(); err != nil {
		log.Crit("Failed to start engine", "error", err)
	}

	// Log configuration
	logConfiguration(config)

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Stats logger
	statsTicker := time.NewTicker(30 * time.Second)
	defer statsTicker.Stop()

	for {
		select {
		case <-sigCh:
			log.Info("📡 Received shutdown signal")
			eng.Stop()
			return

		case <-statsTicker.C:
			stats := eng.GetStats()
			log.Info("📊 Engine Stats",
				"slotsProcessed", stats.SlotsProcessed,
				"blocksProposed", stats.BlocksProposed,
				"missedSlots", stats.MissedSlots,
				"avgSlotTime", stats.AverageSlotTime,
				"uptime", stats.Uptime.Round(time.Second))
		}
	}
}

func detectBlockPeriod(config *engine.Config) uint64 {
	rpc := engine.NewRPCClient(config.ExecutionEndpoint, config.RPCEndpoint, config.JWTSecretPath)

	result, err := rpc.CallRPC("equa_getBlockPeriod", []interface{}{})
	if err != nil {
		log.Warn("Failed to detect block period, using default", "error", err)
		return 12 // Default 12 seconds
	}

	period, ok := result.(float64)
	if !ok {
		log.Warn("Invalid block period format, using default")
		return 12
	}

	return uint64(period)
}

func readJWTSecret(path string) string {
	if path == "" {
		log.Warn("No JWT secret provided, Engine API calls may fail")
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		log.Warn("Failed to read JWT secret", "error", err)
		return ""
	}

	// Trim whitespace and newlines
	secret := strings.TrimSpace(string(data))

	// Remove 0x prefix if present
	if strings.HasPrefix(secret, "0x") {
		secret = secret[2:]
	}

	return secret
}

func logConfiguration(config *engine.Config) {
	log.Info("⚙️  Configuration:")
	log.Info("  Network:")
	log.Info("    Chain ID:", "value", config.ChainID)
	log.Info("    Execution:", "value", config.ExecutionEndpoint)
	log.Info("    RPC:", "value", config.RPCEndpoint)
	log.Info("  Consensus:")
	log.Info("    Slot Duration:", "value", config.SlotDuration)
	log.Info("    Slots/Epoch:", "value", config.SlotsPerEpoch)
	log.Info("    Min Validators:", "value", config.MinValidators)
	log.Info("    Min Stake:", "value", config.MinStake)
	log.Info("  Finality:")
	log.Info("    Threshold:", "value", fmt.Sprintf("%.1f%%", config.FinalityThreshold*100))
	log.Info("    Justification:", "value", fmt.Sprintf("%d slots", config.JustificationDelay))
	log.Info("    Finalization:", "value", fmt.Sprintf("%d slots", config.FinalizationDelay))
	log.Info("  Rewards:")
	log.Info("    Base/Epoch:", "value", config.BaseRewardPerEpoch)
	log.Info("    MEV Bonus:", "value", fmt.Sprintf("%.1f%%", config.MEVBonusMultiplier*100))
	log.Info("    Ordering Bonus:", "value", fmt.Sprintf("%.1f%%", config.OrderingBonusMultiplier*100))
	log.Info("  Advanced:")
	log.Info("    PoW Influence:", "value", fmt.Sprintf("%.1f%%", config.PoWInfluence*100))
	log.Info("    Min Reputation:", "value", config.MinReputationScore)
	log.Info("  Validator:")
	log.Info("    Address:", "value", config.ValidatorAddress)
}
