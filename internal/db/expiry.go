package db

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// MaxFindLimit is the maximum number of records to return in a find operation
	// to prevent large result sets
	MaxFindLimit = 200
)

func (db *Database) FindExpiredDelegations(ctx context.Context, btcTipHeight uint64) ([]model.TimeLockDocument, error) {
	client := db.client.Database(db.dbName).Collection(model.TimeLockCollection)
	filter := bson.M{"expire_height": bson.M{"$lte": btcTipHeight}}

	opts := options.Find().SetLimit(MaxFindLimit) // to prevent large result sets
	cursor, err := client.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			log.Error().Err(err).Msg("failed to close cursor")
		}
	}()

	var delegations []model.TimeLockDocument
	if err = cursor.All(ctx, &delegations); err != nil {
		return nil, err
	}

	return delegations, nil
}

func (db *Database) DeleteExpiredDelegation(ctx context.Context, id primitive.ObjectID) error {
	client := db.client.Database(db.dbName).Collection(model.TimeLockCollection)
	filter := bson.M{"_id": id}

	result, err := client.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete expired delegation with ID %v: %w", id, err)
	}

	// Check if any document was deleted
	if result.DeletedCount == 0 {
		return fmt.Errorf("no expired delegation found with ID %v", id)
	}

	return nil
}

func (db *Database) SaveTimeLockExpireCheck(
	ctx context.Context, stakingTxHashHex string,
	expireHeight uint64, txType string,
) error {
	client := db.client.Database(db.dbName).Collection(model.TimeLockCollection)
	document := model.NewTimeLockDocument(stakingTxHashHex, expireHeight, txType)
	_, err := client.InsertOne(ctx, document)
	if err != nil {
		return err
	}
	return nil
}
