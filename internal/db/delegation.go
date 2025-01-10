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

type BatchResult struct {
	Delegations     []*model.DelegationDocument
	LastProcessedID string
	IsLastBatch     bool
}

func (db *Database) GetBTCDelegationsByStatesInBatches(
	ctx context.Context,
	states []types.DelegationState,
	lastProcessedID string,
	batchSize int64,
) (*BatchResult, error) {
	if batchSize <= 0 {
		batchSize = 500 // Default batch size
	}

	stateStrings := make([]string, len(states))
	for i, state := range states {
		stateStrings[i] = state.ToString()
	}

	filter := bson.M{
		"state": bson.M{"$in": stateStrings},
	}
	if lastProcessedID != "" {
		filter["_id"] = bson.M{"$gt": lastProcessedID}
	}

	// Mongo internally always apply sorting before applying limit
	// this is necessary for pagination to work correctly
	// Order of options builder does not matter
	opts := options.Find().
		SetLimit(batchSize).
		SetSort(bson.M{"_id": 1})

	cursor, err := db.client.Database(db.dbName).
		Collection(model.DelegationsCollection).
		Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var delegations []*model.DelegationDocument
	if err := cursor.All(ctx, &delegations); err != nil {
		return nil, err
	}

	var lastProcessedStakingTxHashHex string
	if len(delegations) > 0 {
		lastProcessedStakingTxHashHex = delegations[len(delegations)-1].StakingTxHashHex
	}

	return &BatchResult{
		Delegations:     delegations,
		LastProcessedID: lastProcessedStakingTxHashHex,
		IsLastBatch:     len(delegations) < int(batchSize),
	}, nil
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
