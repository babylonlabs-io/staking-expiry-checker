package testutil

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// InsertDelegationDirectly inserts a delegation document directly into MongoDB
// This is a test utility function that bypasses the application's DbInterface
func InsertDelegationDirectly(
	ctx context.Context,
	client *mongo.Client,
	dbName string,
	delegation *model.DelegationDocument,
) error {
	collection := client.Database(dbName).Collection(model.DelegationsCollection)
	opts := options.Replace().SetUpsert(true)

	_, err := collection.ReplaceOne(
		ctx,
		bson.M{"_id": delegation.StakingTxHashHex},
		delegation,
		opts,
	)

	return err
}
