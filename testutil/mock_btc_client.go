package testutil

import (
	"fmt"
	"sync"
	"time"
)

// MockBTCClient is a mock implementation of the BTC client for testing
type MockBTCClient struct {
	mu           sync.RWMutex
	blockCount   int64
	blockTimes   map[uint64]int64
	spentUTXOs   map[string]bool
	errGetBlock  error
	errBlockTime error
	errUTXOSpent error
}

// NewMockBTCClient creates a new mock BTC client with default values
func NewMockBTCClient() *MockBTCClient {
	return &MockBTCClient{
		blockCount: 780000, // Example recent Bitcoin height
		blockTimes: make(map[uint64]int64),
		spentUTXOs: make(map[string]bool),
	}
}

// GetBlockCount returns the current block count
func (c *MockBTCClient) GetBlockCount() (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.errGetBlock != nil {
		return 0, c.errGetBlock
	}
	return c.blockCount, nil
}

// GetBlockTimestamp returns the timestamp for a given block height
func (c *MockBTCClient) GetBlockTimestamp(height uint64) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.errBlockTime != nil {
		return 0, c.errBlockTime
	}

	// Return stored timestamp if available
	if ts, ok := c.blockTimes[height]; ok {
		return ts, nil
	}

	// Return a default timestamp based on height (10 min per block)
	now := time.Now().Unix()
	diff := c.blockCount - int64(height)
	return now - diff*600, nil
}

// IsUTXOSpent checks if a UTXO is spent
func (c *MockBTCClient) IsUTXOSpent(txid string, vout uint32) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.errUTXOSpent != nil {
		return false, c.errUTXOSpent
	}

	key := fmt.Sprintf("%s:%d", txid, vout)
	return c.spentUTXOs[key], nil
}

// SetBlockCount sets the current block count
func (c *MockBTCClient) SetBlockCount(count int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.blockCount = count
}

// SetBlockTimestamp sets the timestamp for a specific block height
func (c *MockBTCClient) SetBlockTimestamp(height uint64, timestamp int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.blockTimes[height] = timestamp
}

// SetUTXOSpent marks a UTXO as spent or unspent
func (c *MockBTCClient) SetUTXOSpent(txid string, vout uint32, spent bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := fmt.Sprintf("%s:%d", txid, vout)
	c.spentUTXOs[key] = spent
}

// SetError sets an error to be returned by the specified method
func (c *MockBTCClient) SetError(method string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch method {
	case "GetBlockCount":
		c.errGetBlock = err
	case "GetBlockTimestamp":
		c.errBlockTime = err
	case "IsUTXOSpent":
		c.errUTXOSpent = err
	}
}
