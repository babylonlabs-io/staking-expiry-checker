package db

import (
	"context"
	"errors"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

func (db *Database) TransitionToUnbonded(
	ctx context.Context,
	stakingTxHashHex string,
	unbondTxType types.TransactionType,
) error {
	eligiblePreviousStates := qualifiedStatesToUnbonded(unbondTxType)
	return db.transitionState(
		ctx, stakingTxHashHex, model.Unbonded,
		eligiblePreviousStates,
	)
}

func qualifiedStatesToUnbonded(unbondTxType types.TransactionType) []model.DelegationState {
	switch unbondTxType {
	case types.TransactionTypeActive:
		return []model.DelegationState{model.Active}
	case types.TransactionTypeUnbonding:
		return []model.DelegationState{model.Unbonding}
	default:
		return nil
	}
}

// TransitionState updates the state of a staking transaction to a new state
// It returns an NotFoundError if the staking transaction is not found or not
// in the eligible state to transition
func (db *Database) transitionState(
	ctx context.Context,
	stakingTxHashHex string,
	newState model.DelegationState,
	eligiblePreviousState []model.DelegationState,
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
