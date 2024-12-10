package db

import (
	"context"
	"fmt"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
)

const STAKING_PARAMS_TYPE = "staking_params"

func (db *Database) GetStakingParams(ctx context.Context, version uint32) (*model.StakingParams, error) {
	collection := db.client.Database(db.dbName).
		Collection(model.GlobalParamsCollection)

	filter := bson.M{
		"type":    STAKING_PARAMS_TYPE,
		"version": version,
	}

	var params model.StakingParamsDocument
	err := collection.FindOne(ctx, filter).Decode(&params)
	if err != nil {
		return nil, fmt.Errorf("failed to get staking params: %w", err)
	}

	return params.Params, nil
}
