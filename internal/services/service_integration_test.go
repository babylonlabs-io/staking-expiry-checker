//go:build integration

package services_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/babylonlabs-io/staking-expiry-checker/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dbClient db.DbInterface
var mongoContainer *testutil.MongoDBContainer

// Sets up the test environment before running tests and cleans up afterward
func TestMain(m *testing.M) {
	var err error

	// Initialize MongoDB container for integration tests
	mongoContainer, err = testutil.SetupMongoContainer()
	if err != nil {
		log.Fatalf("Failed to set up MongoDB container: %v", err)
	}

	// Ensure container is cleaned up after tests complete
	defer func() {
		if err := mongoContainer.Cleanup(); err != nil {
			log.Printf("Failed to clean up MongoDB container: %v", err)
		}
	}()

	// Create database connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbClient, err = db.New(ctx, mongoContainer.Config())
	if err != nil {
		log.Fatalf("Failed to create database client: %v", err)
	}

	// Run all tests and exit with appropriate code
	code := m.Run()
	os.Exit(code)
}

// resetDB clears all database collections between tests to ensure isolation
func resetDB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := mongoContainer.ResetDatabase(ctx)
	require.NoError(t, err, "Failed to reset database")
}

// TestDelegationOperations tests the full lifecycle of a delegation:
// active -> unbonding -> unbonded -> withdrawn
func TestDelegationOperations(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Step 1: Create and store an active delegation
	activeDelegation, err := testutil.GenerateRandomDelegation(types.Active)
	require.NoError(t, err)

	mongoClient, err := mongoContainer.GetClient(ctx)
	require.NoError(t, err)
	defer mongoClient.Disconnect(ctx)

	err = testutil.InsertDelegationDirectly(ctx, mongoClient, mongoContainer.GetDatabaseName(), activeDelegation)
	require.NoError(t, err)

	// Step 2: Prepare data for unbonding transition
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

	// Step 3: Transition delegation from active to unbonding state
	err = dbClient.TransitionToUnbondingState(
		ctx,
		activeDelegation.StakingTxHashHex,
		unbondingStartHeight,
		unbondingTimelock,
		unbondingOutputIndex,
		unbondingTxHex,
		unbondingStartTimestamp,
	)
	require.NoError(t, err)

	// Step 4: Verify delegation is now in unbonding state with correct data
	updatedDelegation, err := dbClient.GetBTCDelegationByStakingTxHash(ctx, activeDelegation.StakingTxHashHex)
	require.NoError(t, err)
	assert.Equal(t, types.Unbonding, updatedDelegation.State)
	assert.NotNil(t, updatedDelegation.UnbondingTx)
	assert.Equal(t, unbondingTimelock, updatedDelegation.UnbondingTx.TimeLock)

	// Step 5: Transition delegation from unbonding to unbonded state
	err = dbClient.TransitionToUnbondedState(
		ctx,
		activeDelegation.StakingTxHashHex,
		[]types.DelegationState{types.Unbonding},
	)
	require.NoError(t, err)

	// Step 6: Verify delegation is now in unbonded state
	updatedDelegation, err = dbClient.GetBTCDelegationByStakingTxHash(ctx, activeDelegation.StakingTxHashHex)
	require.NoError(t, err)
	assert.Equal(t, types.Unbonded, updatedDelegation.State)

	// Step 7: Transition delegation from unbonded to withdrawn state
	err = dbClient.TransitionToWithdrawnState(
		ctx,
		activeDelegation.StakingTxHashHex,
		[]types.DelegationState{types.Unbonded},
	)
	require.NoError(t, err)

	// Step 8: Verify delegation is now in withdrawn state (final state)
	updatedDelegation, err = dbClient.GetBTCDelegationByStakingTxHash(ctx, activeDelegation.StakingTxHashHex)
	require.NoError(t, err)
	assert.Equal(t, types.Withdrawn, updatedDelegation.State)
}

// TestTimeLockOperations tests the timelock expiry detection and state transition
// when a delegation's timelock has expired
func TestTimeLockOperations(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Set up test data with an expired timelock
	currentHeight, err := testutil.RandomBlockHeight(0, 0)
	require.NoError(t, err)
	expiryHeight := currentHeight - 10 // Expired 10 blocks ago

	// Create a random staking transaction hash
	stakingTxHash, err := testutil.RandomTxHash()
	require.NoError(t, err)
	txType := "staking"

	// Create and save an active delegation with our test staking tx hash
	activeDelegation, err := testutil.GenerateRandomDelegation(types.Active)
	require.NoError(t, err)
	activeDelegation.StakingTxHashHex = stakingTxHash

	// Get MongoDB client for direct insertion
	mongoClient, err := mongoContainer.GetClient(ctx)
	require.NoError(t, err)
	defer mongoClient.Disconnect(ctx)

	err = testutil.InsertDelegationDirectly(ctx, mongoClient, mongoContainer.GetDatabaseName(), activeDelegation)
	require.NoError(t, err)

	// Create a timelock document that should be detected as expired
	expiredDoc := model.TimeLockDocument{
		StakingTxHashHex: stakingTxHash,
		ExpireHeight:     expiryHeight,
		TxType:           txType,
	}
	err = dbClient.SaveTimeLockExpireCheck(ctx, expiredDoc.StakingTxHashHex, expiredDoc.ExpireHeight, expiredDoc.TxType)
	require.NoError(t, err)

	// Find expired delegations and verify our test delegation is included
	expiredDocs, err := dbClient.FindExpiredDelegations(ctx, currentHeight)
	require.NoError(t, err)
	var found bool
	for _, doc := range expiredDocs {
		if doc.StakingTxHashHex == stakingTxHash {
			found = true
			break
		}
	}
	assert.True(t, found, "Expired document not found for the given stakingTxHash")

	// Transition the expired delegation directly to unbonded state
	err = dbClient.TransitionToUnbondedState(ctx, stakingTxHash, []types.DelegationState{types.Active})
	require.NoError(t, err)

	// Verify the delegation state was updated correctly
	updatedDelegation, err := dbClient.GetBTCDelegationByStakingTxHash(ctx, stakingTxHash)
	require.NoError(t, err)
	assert.Equal(t, types.Unbonded, updatedDelegation.State)
}
