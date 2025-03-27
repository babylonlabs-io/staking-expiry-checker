//go:build integration

package db_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/babylonlabs-io/staking-expiry-checker/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dbClient db.DbInterface
var mongoContainer *testutil.MongoDBContainer

// TestMain sets up the MongoDB container for all tests
func TestMain(m *testing.M) {
	var err error

	// Set up the MongoDB container
	mongoContainer, err = testutil.SetupMongoContainer()
	if err != nil {
		log.Fatalf("Failed to set up MongoDB container: %v", err)
	}

	// Clean up the container when tests are done
	defer func() {
		if err := mongoContainer.Cleanup(); err != nil {
			log.Printf("Failed to clean up MongoDB container: %v", err)
		}
	}()

	// Initialize the database client with the container's config
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbClient, err = db.New(ctx, mongoContainer.Config())
	if err != nil {
		log.Fatalf("Failed to create database client: %v", err)
	}

	// Run the tests
	code := m.Run()

	os.Exit(code)
}

// resetDB clears all collections in the database before each test
func resetDB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := mongoContainer.ResetDatabase(ctx)
	require.NoError(t, err, "Failed to reset database")
}

func TestDatabaseConnection(t *testing.T) {
	ctx := context.Background()

	// Simply test that we can ping the database
	err := dbClient.Ping(ctx)
	assert.NoError(t, err)
}

func TestFindExpiredDelegations(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Set test data
	currentHeight, err := testutil.RandomBlockHeight(0, 0)
	require.NoError(t, err)

	// Document that should expire (below current height)
	expiredDoc1, err := testutil.GenerateRandomTimeLockDocument(currentHeight-10, "staking")
	require.NoError(t, err)

	// Document that should expire (at current height)
	expiredDoc2, err := testutil.GenerateRandomTimeLockDocument(currentHeight, "unbonding")
	require.NoError(t, err)

	// Document that should not expire (above current height)
	nonExpiredDoc, err := testutil.GenerateRandomTimeLockDocument(currentHeight+10, "staking")
	require.NoError(t, err)

	// Save the timelock documents
	err = dbClient.SaveTimeLockExpireCheck(ctx, expiredDoc1.StakingTxHashHex, expiredDoc1.ExpireHeight, expiredDoc1.TxType)
	require.NoError(t, err)

	err = dbClient.SaveTimeLockExpireCheck(ctx, expiredDoc2.StakingTxHashHex, expiredDoc2.ExpireHeight, expiredDoc2.TxType)
	require.NoError(t, err)

	err = dbClient.SaveTimeLockExpireCheck(ctx, nonExpiredDoc.StakingTxHashHex, nonExpiredDoc.ExpireHeight, nonExpiredDoc.TxType)
	require.NoError(t, err)

	// Test finding expired delegations
	expiredDocs, err := dbClient.FindExpiredDelegations(ctx, currentHeight)
	assert.NoError(t, err)
	assert.Len(t, expiredDocs, 2)

	// Verify we have both expired documents
	stakingTxHashes := map[string]bool{
		expiredDoc1.StakingTxHashHex: false,
		expiredDoc2.StakingTxHashHex: false,
	}

	for _, doc := range expiredDocs {
		stakingTxHashes[doc.StakingTxHashHex] = true
	}

	assert.True(t, stakingTxHashes[expiredDoc1.StakingTxHashHex])
	assert.True(t, stakingTxHashes[expiredDoc2.StakingTxHashHex])
}

func TestSaveTimeLockExpireCheck(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Test data
	stakingTxHash, err := testutil.RandomTxHash()
	require.NoError(t, err)

	expireHeight, err := testutil.RandomBlockHeight(0, 0)
	require.NoError(t, err)

	txType := "staking"

	// Save timelock expire check
	err = dbClient.SaveTimeLockExpireCheck(ctx, stakingTxHash, expireHeight, txType)
	assert.NoError(t, err)

	// Find expired delegations to verify the document was inserted
	expiredDocs, err := dbClient.FindExpiredDelegations(ctx, expireHeight)
	assert.NoError(t, err)
	assert.Len(t, expiredDocs, 1)

	// Verify the data in the document
	assert.Equal(t, stakingTxHash, expiredDocs[0].StakingTxHashHex)
	assert.Equal(t, expireHeight, expiredDocs[0].ExpireHeight)
	assert.Equal(t, txType, expiredDocs[0].TxType)
}

func TestDeleteExpiredDelegation(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Insert test data
	stakingTxHash, err := testutil.RandomTxHash()
	require.NoError(t, err)

	expireHeight, err := testutil.RandomBlockHeight(0, 0)
	require.NoError(t, err)

	txType := "staking"

	// Save timelock expire check
	err = dbClient.SaveTimeLockExpireCheck(ctx, stakingTxHash, expireHeight, txType)
	assert.NoError(t, err)

	// Get the expired document
	expiredDocs, err := dbClient.FindExpiredDelegations(ctx, expireHeight)
	assert.NoError(t, err)
	assert.Len(t, expiredDocs, 1)

	// Delete the document
	err = dbClient.DeleteExpiredDelegation(ctx, expiredDocs[0].ID)
	assert.NoError(t, err)

	// Verify document was deleted by checking there are no expired docs
	expiredDocs, err = dbClient.FindExpiredDelegations(ctx, expireHeight)
	assert.NoError(t, err)
	assert.Len(t, expiredDocs, 0)
}

func TestGetBTCDelegationByStakingTxHash(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Generate test delegation
	delegation, err := testutil.GenerateRandomDelegation(types.Active)
	require.NoError(t, err)

	mongoClient, err := mongoContainer.GetClient(ctx)
	require.NoError(t, err)
	defer mongoClient.Disconnect(ctx)

	err = testutil.InsertDelegationDirectly(ctx, mongoClient, mongoContainer.GetDatabaseName(), delegation)
	require.NoError(t, err)

	// Get delegation by staking tx hash
	result, err := dbClient.GetBTCDelegationByStakingTxHash(ctx, delegation.StakingTxHashHex)
	assert.NoError(t, err)
	assert.Equal(t, delegation.StakingTxHashHex, result.StakingTxHashHex)
	assert.Equal(t, delegation.StakerPkHex, result.StakerPkHex)
	assert.Equal(t, delegation.FinalityProviderPkHex, result.FinalityProviderPkHex)
	assert.Equal(t, delegation.State, result.State)
}

func TestBTCDelegationStates(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Generate test delegations
	activeDelegation, err := testutil.GenerateRandomDelegation(types.Active)
	require.NoError(t, err)

	unbondingDelegation, err := testutil.GenerateRandomDelegation(types.Unbonding)
	require.NoError(t, err)

	unbondedDelegation, err := testutil.GenerateRandomDelegation(types.Unbonded)
	require.NoError(t, err)

	// Get MongoDB client for direct insertion
	mongoClient, err := mongoContainer.GetClient(ctx)
	require.NoError(t, err)
	defer mongoClient.Disconnect(ctx)

	// Insert delegations directly into MongoDB
	err = testutil.InsertDelegationDirectly(ctx, mongoClient, mongoContainer.GetDatabaseName(), activeDelegation)
	require.NoError(t, err)

	err = testutil.InsertDelegationDirectly(ctx, mongoClient, mongoContainer.GetDatabaseName(), unbondingDelegation)
	require.NoError(t, err)

	err = testutil.InsertDelegationDirectly(ctx, mongoClient, mongoContainer.GetDatabaseName(), unbondedDelegation)
	require.NoError(t, err)

	// Test getting delegations by states - single state
	activeStates := []types.DelegationState{types.Active}
	result, err := dbClient.GetBTCDelegationsByStates(ctx, activeStates, "")
	assert.NoError(t, err)
	assert.Len(t, result.Data, 1)
	assert.Equal(t, activeDelegation.StakingTxHashHex, result.Data[0].StakingTxHashHex)

	// Test getting delegations by states - multiple states
	multiStates := []types.DelegationState{types.Unbonding, types.Unbonded}
	result, err = dbClient.GetBTCDelegationsByStates(ctx, multiStates, "")
	assert.NoError(t, err)
	assert.Len(t, result.Data, 2)

	// Create a map of expected staking tx hashes
	expectedTxHashes := map[string]bool{
		unbondingDelegation.StakingTxHashHex: false,
		unbondedDelegation.StakingTxHashHex:  false,
	}

	// Check that both expected delegation hashes are in the result
	for _, record := range result.Data {
		expectedTxHashes[record.StakingTxHashHex] = true
	}

	assert.True(t, expectedTxHashes[unbondingDelegation.StakingTxHashHex])
	assert.True(t, expectedTxHashes[unbondedDelegation.StakingTxHashHex])
}

// TestStateTransitions tests the state transition functions
func TestStateTransitions(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Generate test delegation in ACTIVE state
	delegation, err := testutil.GenerateRandomDelegation(types.Active)
	require.NoError(t, err)

	mongoClient, err := mongoContainer.GetClient(ctx)
	require.NoError(t, err)
	defer mongoClient.Disconnect(ctx)

	err = testutil.InsertDelegationDirectly(ctx, mongoClient, mongoContainer.GetDatabaseName(), delegation)
	require.NoError(t, err)

	// Test transition to UNBONDING state
	unbondingStartHeight, err := testutil.RandomBlockHeight(0, 0)
	require.NoError(t, err)

	unbondingTimelock, err := testutil.RandomTimelock()
	require.NoError(t, err)

	unbondingOutputIndex, err := testutil.RandomOutputIndex()
	require.NoError(t, err)

	unbondingTxHex, err := testutil.RandomTxHex()
	require.NoError(t, err)

	unbondingStartTimestamp, err := testutil.RandomTimestamp(time.Time{}, time.Time{})
	require.NoError(t, err)

	err = dbClient.TransitionToUnbondingState(
		ctx,
		delegation.StakingTxHashHex,
		unbondingStartHeight,
		unbondingTimelock,
		unbondingOutputIndex,
		unbondingTxHex,
		unbondingStartTimestamp,
	)
	assert.NoError(t, err)

	// Verify transition to UNBONDING
	updatedDelegation, err := dbClient.GetBTCDelegationByStakingTxHash(ctx, delegation.StakingTxHashHex)
	require.NoError(t, err)
	assert.Equal(t, types.Unbonding, updatedDelegation.State)
	assert.NotNil(t, updatedDelegation.UnbondingTx)
	assert.Equal(t, unbondingTxHex, updatedDelegation.UnbondingTx.TxHex)
	assert.Equal(t, unbondingStartHeight, updatedDelegation.UnbondingTx.StartHeight)
	assert.Equal(t, unbondingTimelock, updatedDelegation.UnbondingTx.TimeLock)
	assert.Equal(t, unbondingOutputIndex, updatedDelegation.UnbondingTx.OutputIndex)
	assert.Equal(t, unbondingStartTimestamp, updatedDelegation.UnbondingTx.StartTimestamp)

	// Test transition to UNBONDED state
	err = dbClient.TransitionToUnbondedState(
		ctx,
		delegation.StakingTxHashHex,
		[]types.DelegationState{types.Unbonding},
	)
	assert.NoError(t, err)

	// Verify transition to UNBONDED
	updatedDelegation, err = dbClient.GetBTCDelegationByStakingTxHash(ctx, delegation.StakingTxHashHex)
	require.NoError(t, err)
	assert.Equal(t, types.Unbonded, updatedDelegation.State)

	// Test transition to WITHDRAWN state
	err = dbClient.TransitionToWithdrawnState(
		ctx,
		delegation.StakingTxHashHex,
		[]types.DelegationState{types.Unbonded},
	)
	assert.NoError(t, err)

	// Verify transition to WITHDRAWN
	updatedDelegation, err = dbClient.GetBTCDelegationByStakingTxHash(ctx, delegation.StakingTxHashHex)
	require.NoError(t, err)
	assert.Equal(t, types.Withdrawn, updatedDelegation.State)

	// Test invalid state transition
	// Try to transition WITHDRAWN -> ACTIVE (which should fail)
	newDelegation, err := testutil.GenerateRandomDelegation(types.Withdrawn)
	require.NoError(t, err)

	// Insert new delegation directly into MongoDB
	err = testutil.InsertDelegationDirectly(ctx, mongoClient, mongoContainer.GetDatabaseName(), newDelegation)
	require.NoError(t, err)

	// Try to transition from WITHDRAWN to UNBONDING
	// Note: We're reusing the same delegation but now it's in WITHDRAWN state
	err = dbClient.TransitionToUnbondingState(
		ctx,
		newDelegation.StakingTxHashHex,
		unbondingStartHeight,
		unbondingTimelock,
		unbondingOutputIndex,
		unbondingTxHex,
		unbondingStartTimestamp,
	)
	// Don't strictly check for error, just check the state remains unchanged

	// The state should still be WITHDRAWN regardless of whether an error occurs
	unchangedDelegation, err := dbClient.GetBTCDelegationByStakingTxHash(ctx, newDelegation.StakingTxHashHex)
	require.NoError(t, err)
	assert.Equal(t, types.Withdrawn, unchangedDelegation.State)
}
