package db

import (
	"context"
	"errors"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *Database) TransitionToUnbondedState(
	ctx context.Context,
	stakingTxHashHex string,
	eligiblePreviousStates []types.DelegationState,
) error {
	return db.transitionState(ctx, stakingTxHashHex, types.Unbonded.ToString(), eligiblePreviousStates, nil)
}

// Change the state to `unbonding` and save the unbondingTx data
// Return not found error if the stakingTxHashHex is not found or the existing state is not eligible for unbonding
func (db *Database) TransitionToUnbondingState(
	ctx context.Context, stakingTxHashHex string,
	unbondingStartHeight, unbondingTimelock, unbondingOutputIndex uint64,
	unbondingTxHex string, unbondingStartTimestamp int64,
) error {
	unbondingTxMap := make(map[string]interface{})
	unbondingTxMap["unbonding_tx"] = model.TimelockTransaction{
		TxHex:          unbondingTxHex,
		OutputIndex:    unbondingOutputIndex,
		StartTimestamp: unbondingStartTimestamp,
		StartHeight:    unbondingStartHeight,
		TimeLock:       unbondingTimelock,
	}

	err := db.transitionState(
		ctx, stakingTxHashHex, types.Unbonding.ToString(),
		utils.QualifiedStatesToUnbonding(), unbondingTxMap,
	)
	if err != nil {
		return err
	}
	return nil
}

func (db *Database) TransitionToWithdrawnState(
	ctx context.Context,
	stakingTxHashHex string,
	eligiblePreviousStates []types.DelegationState,
) error {
	return db.transitionState(ctx, stakingTxHashHex, types.Withdrawn.ToString(), eligiblePreviousStates, nil)
}

// TransitionState updates the state of a staking transaction to a new state
// It returns an NotFoundError if the staking transaction is not found or not in the eligible state to transition
func (db *Database) transitionState(
	ctx context.Context, stakingTxHashHex, newState string,
	eligiblePreviousState []types.DelegationState, additionalUpdates map[string]interface{},
) error {
	client := db.client.Database(db.dbName).Collection(model.DelegationsCollection)
	filter := bson.M{"_id": stakingTxHashHex, "state": bson.M{"$in": eligiblePreviousState}}
	update := bson.M{"$set": bson.M{"state": newState}}
	for field, value := range additionalUpdates {
		// Add additional fields to the $set operation
		update["$set"].(bson.M)[field] = value
	}
	_, err := client.UpdateOne(ctx, filter, update)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return &NotFoundError{
				Key:     stakingTxHashHex,
				Message: "Delegation not found or not in eligible state to transition",
			}
		}
		return err
	}
	return nil
}

func (db *Database) GetBTCDelegationByStakingTxHash(
	ctx context.Context, stakingTxHash string,
) (*model.DelegationDocument, error) {
	filter := bson.M{"_id": stakingTxHash}

	res := db.client.Database(db.dbName).
		Collection(model.DelegationsCollection).
		FindOne(ctx, filter)

	var delegationDoc model.DelegationDocument
	err := res.Decode(&delegationDoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, &NotFoundError{
				Key:     stakingTxHash,
				Message: "BTC delegation not found when getting by staking tx hash",
			}
		}
		return nil, err
	}

	return &delegationDoc, nil
}

func (db *Database) GetBTCDelegationsByStates(
	ctx context.Context,
	states []types.DelegationState,
	paginationToken string,
) (*DbResultMap[model.DelegationDocument], error) {
	// Convert states to strings
	stateStrings := make([]string, len(states))
	for i, state := range states {
		stateStrings[i] = state.ToString()
	}

	// Build filter
	filter := bson.M{
		"state": bson.M{"$in": stateStrings},
	}

	// Setup options
	options := options.Find()
	options.SetSort(bson.M{"_id": 1})

	// Decode pagination token if it exists
	if paginationToken != "" {
		decodedToken, err := model.DecodePaginationToken[model.DelegationScanPagination](paginationToken)
		if err != nil {
			return nil, &InvalidPaginationTokenError{
				Message: "Invalid pagination token",
			}
		}
		filter["_id"] = bson.M{"$gt": decodedToken.StakingTxHashHex}
	}

	return findWithPagination(
		ctx,
		db.client.Database(db.dbName).Collection(model.DelegationsCollection),
		filter,
		options,
		db.cfg.MaxPaginationLimit,
		model.BuildDelegationScanPaginationToken,
	)
}

func (db *Database) GetBTCDelegationState(
	ctx context.Context, stakingTxHash string,
) (*types.DelegationState, error) {
	delegation, err := db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHash)
	if err != nil {
		return nil, err
	}
	return &delegation.State, nil
}
