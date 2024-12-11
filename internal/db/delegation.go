package db

import (
	"context"
	"errors"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (db *Database) TransitionToUnbondedState(
	ctx context.Context,
	stakingTxHashHex string,
	unbondTxType types.StakingTxType,
) error {
	return db.transitionState(
		ctx, stakingTxHashHex, types.Unbonded,
		utils.QualifiedStatesToUnbonded(unbondTxType),
	)
}

func (db *Database) TransitionToUnbondingState(
	ctx context.Context,
	stakingTxHashHex string,
) error {
	return db.transitionState(
		ctx, stakingTxHashHex, types.Unbonding,
		utils.QualifiedStatesToUnbonding(),
	)
}

func (db *Database) TransitionToWithdrawnState(ctx context.Context, stakingTxHashHex string) error {
	err := db.transitionState(
		ctx, stakingTxHashHex, types.Withdrawn,
		utils.QualifiedStatesToWithdraw(),
	)
	if err != nil {
		return err
	}
	return nil
}

// TransitionState updates the state of a staking transaction to a new state
// It returns an NotFoundError if the staking transaction is not found or not
// in the eligible state to transition
func (db *Database) transitionState(
	ctx context.Context,
	stakingTxHashHex string,
	newState types.DelegationState,
	eligiblePreviousState []types.DelegationState,
) error {
	client := db.client.Database(
		db.dbName,
	).Collection(model.DelegationsCollection)
	filter := bson.M{
		"_id": stakingTxHashHex,
		"state": bson.M{
			"$in": eligiblePreviousState,
		},
	}
	update := bson.M{"$set": bson.M{"state": newState.ToString()}}
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
) (*model.BTCDelegationDetails, error) {
	filter := bson.M{"_id": stakingTxHash}

	res := db.client.Database(db.dbName).
		Collection(model.DelegationsCollection).
		FindOne(ctx, filter)

	var delegationDoc model.BTCDelegationDetails
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
) ([]*model.BTCDelegationDetails, error) {
	// Convert states to a slice of strings
	stateStrings := make([]string, len(states))
	for i, state := range states {
		stateStrings[i] = state.ToString()
	}

	filter := bson.M{"state": bson.M{"$in": stateStrings}}

	cursor, err := db.client.Database(db.dbName).
		Collection(model.DelegationsCollection).
		Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var delegations []*model.BTCDelegationDetails
	if err := cursor.All(ctx, &delegations); err != nil {
		return nil, err
	}

	return delegations, nil
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
