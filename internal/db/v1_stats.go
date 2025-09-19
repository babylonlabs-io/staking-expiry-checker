package db

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CalculateAndUpsertV1OverallStats calculates Phase-1 overall stats by scanning
// the delegations collection and upserts the results to the simplified stats collection.
// This method replaces the incremental stats approach for Phase-1 delegations.
func (db *Database) CalculateAndUpsertV1OverallStats(
	ctx context.Context,
) (*model.V1OverallStatsDocument, error) {
	delegationClient := db.client.Database(db.dbName).Collection(model.DelegationsCollection)
	statsClient := db.client.Database(db.dbName).Collection(model.V1OverallStatsSimplifiedCollection)

	// Start a session for transaction
	session, sessionErr := db.client.StartSession()
	if sessionErr != nil {
		return nil, sessionErr
	}
	defer session.EndSession(ctx)

	// Define the work to be done in the transaction
	transactionWork := func(sessCtx mongo.SessionContext) (interface{}, error) {
		// Aggregate active delegations: state="active" AND is_overflow=false
		cursor, err := delegationClient.Aggregate(
			sessCtx,
			[]bson.M{
				{
					"$match": bson.M{
						"state":       types.Active.ToString(),
						"is_overflow": false,
					},
				},
				{
					"$group": bson.M{
						"_id":        nil,
						"count":      bson.M{"$sum": 1},
						"totalValue": bson.M{"$sum": "$staking_value"},
					},
				},
			},
		)
		if err != nil {
			return nil, err
		}
		defer cursor.Close(sessCtx)

		type aggregateStats struct {
			Count      int64 `bson:"count"`
			TotalValue int64 `bson:"totalValue"`
		}

		var result aggregateStats
		if cursor.Next(sessCtx) {
			if err := cursor.Decode(&result); err != nil {
				return nil, err
			}
		}

		if err := cursor.Err(); err != nil {
			return nil, err
		}

		statsDoc := model.V1OverallStatsDocument{
			Id:                "singleton",
			ActiveTvl:         result.TotalValue,
			ActiveDelegations: result.Count,
		}

		// Upsert the stats document (replace if exists)
		upsertFilter := bson.M{"_id": "singleton"}
		upsertUpdate := bson.M{
			"$set": bson.M{
				"active_tvl":         statsDoc.ActiveTvl,
				"active_delegations": statsDoc.ActiveDelegations,
			},
		}

		_, err = statsClient.UpdateOne(
			sessCtx,
			upsertFilter,
			upsertUpdate,
			options.Update().SetUpsert(true),
		)
		if err != nil {
			return nil, err
		}

		return &statsDoc, nil
	}

	// Execute the transaction
	result, txErr := session.WithTransaction(ctx, transactionWork)
	if txErr != nil {
		return nil, txErr
	}

	return result.(*model.V1OverallStatsDocument), nil
}
