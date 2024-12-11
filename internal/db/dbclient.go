package db

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
)

type Database struct {
	dbName string
	client *mongo.Client
}

func New(ctx context.Context, cfg config.DbConfig) (*Database, error) {
	credential := options.Credential{
		Username: cfg.Username,
		Password: cfg.Password,
	}
	clientOps := options.Client().ApplyURI(cfg.Address).SetAuth(credential)
	client, err := mongo.Connect(ctx, clientOps)
	if err != nil {
		return nil, err
	}

	return &Database{
		dbName: cfg.DbName,
		client: client,
	}, nil
}

func (db *Database) Ping(ctx context.Context) error {
	err := db.client.Ping(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (db *Database) Shutdown(ctx context.Context) error {
	return db.client.Disconnect(ctx)
}
