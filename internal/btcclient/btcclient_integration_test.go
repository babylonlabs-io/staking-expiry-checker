//go:build integration

package btcclient_test

import (
	"testing"
	"time"

	"github.com/babylonlabs-io/staking-expiry-checker/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockBTCClient(t *testing.T) {
	mockClient := testutil.NewMockBTCClient()

	// Test GetBlockCount
	blockCount, err := mockClient.GetBlockCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(780000), blockCount) // Default value in the mock

	// Update block count and verify
	newBlockCount := int64(785000)
	mockClient.SetBlockCount(newBlockCount)
	blockCount, err = mockClient.GetBlockCount()
	assert.NoError(t, err)
	assert.Equal(t, newBlockCount, blockCount)

	// Test GetBlockTimestamp with default behavior
	height := uint64(779990)
	timestamp, err := mockClient.GetBlockTimestamp(height)
	assert.NoError(t, err)
	// The timestamp should be calculated based on block height (10 mins per block)
	// but we can't predict the exact value as it's based on current time
	// Just ensure it's a reasonable timestamp (not zero, less than now)
	assert.Greater(t, timestamp, int64(0))
	assert.LessOrEqual(t, timestamp, time.Now().Unix())

	// Set a specific timestamp and verify
	specificHeight := uint64(779980)
	specificTimestamp := time.Now().Unix() - 3600 // 1 hour ago
	mockClient.SetBlockTimestamp(specificHeight, specificTimestamp)
	fetchedTimestamp, err := mockClient.GetBlockTimestamp(specificHeight)
	assert.NoError(t, err)
	assert.Equal(t, specificTimestamp, fetchedTimestamp)

	// Test IsUTXOSpent with default behavior (not spent)
	txid, err := testutil.RandomTxHash()
	require.NoError(t, err)
	vout := uint32(0)
	spent, err := mockClient.IsUTXOSpent(txid, vout)
	assert.NoError(t, err)
	assert.False(t, spent) // Default is false (not spent)

	// Mark as spent and verify
	mockClient.SetUTXOSpent(txid, vout, true)
	spent, err = mockClient.IsUTXOSpent(txid, vout)
	assert.NoError(t, err)
	assert.True(t, spent)
}

// TestBTCClientWithError tests error handling in the BTC client
func TestBTCClientWithError(t *testing.T) {
	mockClient := testutil.NewMockBTCClient()

	// Set up errors for different methods
	testError := assert.AnError // Standard error for testing

	// Test GetBlockCount with error
	mockClient.SetError("GetBlockCount", testError)
	_, err := mockClient.GetBlockCount()
	assert.Error(t, err)
	assert.Equal(t, testError, err)

	// Test GetBlockTimestamp with error
	mockClient.SetError("GetBlockTimestamp", testError)
	_, err = mockClient.GetBlockTimestamp(780000)
	assert.Error(t, err)
	assert.Equal(t, testError, err)

	// Test IsUTXOSpent with error
	mockClient.SetError("IsUTXOSpent", testError)
	randomTxid, err := testutil.RandomTxHash()
	require.NoError(t, err)
	_, err = mockClient.IsUTXOSpent(randomTxid, 0)
	assert.Error(t, err)
	assert.Equal(t, testError, err)

	// Reset errors and ensure methods work again
	mockClient.SetError("GetBlockCount", nil)
	mockClient.SetError("GetBlockTimestamp", nil)
	mockClient.SetError("IsUTXOSpent", nil)

	blockCount, err := mockClient.GetBlockCount()
	assert.NoError(t, err)
	assert.Equal(t, int64(780000), blockCount)
}
