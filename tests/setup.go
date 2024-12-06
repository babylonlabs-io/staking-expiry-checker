package tests

import (
	"context"
	"log"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/btcclient"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/observability/metrics"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/poller"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/services"
)

type TestServerDependency struct {
	ConfigOverrides *config.Config
	MockDbClient    db.DbInterface
	MockBtcClient   btcclient.BtcInterface
}

func setupTestServer(t *testing.T, dep *TestServerDependency) (db.DbInterface, btcclient.BtcInterface, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	cfg, err := config.New("./config-test.yml")
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}
	metricsPort := cfg.Metrics.GetMetricsPort()
	metrics.Init(metricsPort)

	if dep != nil && dep.ConfigOverrides != nil {
		applyConfigOverrides(cfg, dep.ConfigOverrides)
	}

	var (
		dbClient  db.DbInterface
		btcClient btcclient.BtcInterface
	)

	if dep != nil && dep.MockBtcClient != nil {
		btcClient = dep.MockBtcClient
	} else {
		btcClient, err = btcclient.NewBtcClient(&cfg.Btc)
		if err != nil {
			t.Fatalf("Failed to initialize btc client: %v", err)
		}
	}

	if dep != nil && dep.MockDbClient != nil {
		dbClient = dep.MockDbClient
	} else {
		setupTestDB(cfg)
		dbClient, err = db.New(ctx, cfg.Db)
		if err != nil {
			t.Fatalf("Failed to initialize db client: %v", err)
		}

	}

	service := services.NewService(dbClient, btcClient)
	p, err := poller.NewPoller(cfg.Poller.Interval, service)
	if err != nil {
		t.Fatalf("Failed to initialize poller: %v", err)
	}

	teardown := func() {
		p.Stop()
		cancel() // Cancel the context to release resources
	}
	return dbClient, btcClient, teardown
}

// Generic function to apply configuration overrides
func applyConfigOverrides(defaultCfg *config.Config, overrides *config.Config) {
	defaultVal := reflect.ValueOf(defaultCfg).Elem()
	overrideVal := reflect.ValueOf(overrides).Elem()

	for i := 0; i < defaultVal.NumField(); i++ {
		defaultField := defaultVal.Field(i)
		overrideField := overrideVal.Field(i)

		if overrideField.IsZero() {
			continue // Skip fields that are not set
		}

		if defaultField.CanSet() {
			defaultField.Set(overrideField)
		}
	}
}

// PurgeAllCollections drops all collections in the specified database.
func PurgeAllCollections(ctx context.Context, client *mongo.Client, databaseName string) error {
	database := client.Database(databaseName)
	collections, err := database.ListCollectionNames(ctx, bson.D{{}})
	if err != nil {
		return err
	}

	for _, collection := range collections {
		if err := database.Collection(collection).Drop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// setupTestDB connects to MongoDB and purges all collections.
func setupTestDB(cfg *config.Config) {
	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(cfg.Db.Address))
	if err != nil {
		log.Fatal(err)
	}

	// Purge all collections in the test database
	if err := PurgeAllCollections(context.TODO(), client, cfg.Db.DbName); err != nil {
		log.Fatal("Failed to purge database:", err)
	}
}

func insertTestDelegations(t *testing.T, docs []model.TimeLockDocument) {
	cfg, err := config.New("./config-test.yml")
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}
	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(cfg.Db.Address))
	if err != nil {
		log.Fatal(err)
	}
	database := client.Database(cfg.Db.DbName)
	collection := database.Collection(model.TimeLockCollection)

	// Convert slice of TimeLockDocument to slice of interface{} for InsertMany
	var documents []interface{}
	for _, doc := range docs {
		documents = append(documents, doc)
	}

	_, err = collection.InsertMany(context.Background(), documents)
	if err != nil {
		t.Fatalf("Failed to insert test delegations: %v", err)
	}
}

func fetchAllTestDelegations(t *testing.T) []model.TimeLockDocument {
	cfg, err := config.New("./config-test.yml")
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}
	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(cfg.Db.Address))
	if err != nil {
		log.Fatal(err)
	}
	database := client.Database(cfg.Db.DbName)
	collection := database.Collection(model.TimeLockCollection)

	cursor, err := collection.Find(context.Background(), bson.D{})
	if err != nil {
		t.Fatalf("Failed to fetch test delegations: %v", err)
	}
	defer cursor.Close(context.Background())

	var results []model.TimeLockDocument
	for cursor.Next(context.Background()) {
		var result model.TimeLockDocument
		err := cursor.Decode(&result)
		if err != nil {
			t.Fatalf("Failed to decode test delegations: %v", err)
		}
		results = append(results, result)
	}

	return results
}
