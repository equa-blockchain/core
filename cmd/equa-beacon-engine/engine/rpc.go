// Copyright 2024 The go-equa Authors
// EQUA Beacon Engine - RPC Client

package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/equa/go-equa/common"
	"github.com/equa/go-equa/log"
)

// RPCClient handles communication with execution layer
type RPCClient struct {
	endpoint    string
	rpcEndpoint string
	client      *http.Client
	jwtToken    string
}

// NewRPCClient creates a new RPC client
func NewRPCClient(endpoint, rpcEndpoint, jwtToken string) *RPCClient {
	return &RPCClient{
		endpoint:    endpoint,
		rpcEndpoint: rpcEndpoint,
		client:      &http.Client{Timeout: 30 * time.Second},
		jwtToken:    jwtToken,
	}
}

// GetPoWQuality gets PoW quality from latest block
func (rpc *RPCClient) GetPoWQuality() (*big.Int, error) {
	result, err := rpc.CallRPC("equa_getPoWDifficulty", []interface{}{})
	if err != nil {
		return nil, err
	}

	// Parse result
	qualityFloat, ok := result.(float64)
	if !ok {
		return nil, fmt.Errorf("unexpected type: %T", result)
	}

	return big.NewInt(int64(qualityFloat)), nil
}

// GetBlockNumberByHash gets block number from hash
func (rpc *RPCClient) GetBlockNumberByHash(hash common.Hash) (uint64, error) {
	result, err := rpc.CallRPC("eth_getBlockByHash", []interface{}{hash.Hex(), false})
	if err != nil {
		return 0, err
	}

	block, ok := result.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid block format")
	}

	numberHex, ok := block["number"].(string)
	if !ok {
		return 0, fmt.Errorf("no number in block")
	}

	number := new(big.Int)
	number.SetString(numberHex[2:], 16)

	return number.Uint64(), nil
}

// GetMEVDetected checks if MEV was detected in block
func (rpc *RPCClient) GetMEVDetected(blockNumber uint64) (bool, error) {
	result, err := rpc.CallRPC("equa_getMEVStats", []interface{}{1})
	if err != nil {
		return false, err
	}

	stats, ok := result.(map[string]interface{})
	if !ok {
		return false, nil
	}

	blocksWithMEV, ok := stats["blocksWithMEV"].(float64)
	if !ok {
		return false, nil
	}

	return blocksWithMEV > 0, nil
}

// GetOrderingScore gets ordering score for block
func (rpc *RPCClient) GetOrderingScore(blockNumber uint64) (*OrderingScoreResult, error) {
	result, err := rpc.CallRPC("equa_getOrderingScore", []interface{}{blockNumber})
	if err != nil {
		return nil, err
	}

	data, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid result format")
	}

	score, _ := data["score"].(float64)
	fairOrdering, _ := data["fairOrdering"].(bool)

	return &OrderingScoreResult{
		Score:        score,
		FairOrdering: fairOrdering,
	}, nil
}

// GetValidators gets validator list
func (rpc *RPCClient) GetValidators() ([]*Validator, error) {
	result, err := rpc.CallRPC("equa_getValidators", []interface{}{})
	if err != nil {
		return nil, err
	}

	validatorList, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid validator list format")
	}

	validators := make([]*Validator, 0, len(validatorList))
	for _, v := range validatorList {
		vMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		addr := common.HexToAddress(vMap["address"].(string))
		stakeStr, _ := vMap["stake"].(string)
		active, _ := vMap["active"].(bool)

		stake := new(big.Int)
		stake.SetString(stakeStr, 10)

		validators = append(validators, &Validator{
			Address: addr,
			Stake:   stake,
			Active:  active,
		})
	}

	return validators, nil
}

// CallRPC makes an RPC call
func (rpc *RPCClient) CallRPC(method string, params []interface{}) (interface{}, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", rpc.rpcEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := rpc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if errObj, ok := result["error"]; ok {
		return nil, fmt.Errorf("RPC error: %v", errObj)
	}

	return result["result"], nil
}

// CallEngine makes Engine API call with JWT auth
func (rpc *RPCClient) CallEngine(method string, params []interface{}) (interface{}, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", rpc.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if rpc.jwtToken != "" {
		req.Header.Set("Authorization", "Bearer "+rpc.jwtToken)
	}

	resp, err := rpc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Debug("Failed to decode response", "body", string(body))
		return nil, err
	}

	if errObj, ok := result["error"]; ok {
		return nil, fmt.Errorf("Engine API error: %v", errObj)
	}

	return result["result"], nil
}

type OrderingScoreResult struct {
	Score        float64
	FairOrdering bool
}
