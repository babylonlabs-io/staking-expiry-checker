package testutil

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	mongoUsername     = "user"
	mongoPassword     = "password"
	mongoDatabaseName = "test-database"

	// Constants for various configurations
	randomContainerNameLength = 6
	mongoConnectTimeout       = 5 * time.Second
)

// MongoDBContainer represents a MongoDB container for testing
type MongoDBContainer struct {
	pool     *dockertest.Pool
	resource *dockertest.Resource
	config   *config.DbConfig
}

// SetupMongoContainer creates and starts a MongoDB container
func SetupMongoContainer() (*MongoDBContainer, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, fmt.Errorf("could not create docker pool: %w", err)
	}

	// Generate random string for container name
	randomString, err := RandomAlphaNum(randomContainerNameLength)
	if err != nil {
		return nil, fmt.Errorf("could not generate random string: %w", err)
	}

	// There can be only 1 container with the same name, so we add
	// random string in the end in case there is still old container running
	containerName := "mongo-integration-tests-db-" + randomString
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       containerName,
		Repository: "mongo",
		Env: []string{
			"MONGO_INITDB_ROOT_USERNAME=" + mongoUsername,
			"MONGO_INITDB_ROOT_PASSWORD=" + mongoPassword,
			"MONGO_INITDB_DATABASE=" + mongoDatabaseName,
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		return nil, fmt.Errorf("could not start resource: %w", err)
	}

	// Get host port (randomly chosen) that is mapped to mongo port inside container
	hostPort := resource.GetPort("27017/tcp")

	dbConfig := &config.DbConfig{
		Username: mongoUsername,
		Password: mongoPassword,
		DbName:   mongoDatabaseName,
		Address:  fmt.Sprintf("mongodb://localhost:%s/", hostPort),
	}

	// Wait for MongoDB to be ready
	if err = pool.Retry(func() error {
		var err error
		ctx, cancel := context.WithTimeout(context.Background(), mongoConnectTimeout)
		defer cancel()

		// Try to connect to MongoDB
		client, err := mongo.Connect(
			ctx,
			options.Client().ApplyURI(dbConfig.Address).SetAuth(options.Credential{
				Username: dbConfig.Username,
				Password: dbConfig.Password,
			}),
		)
		if err != nil {
			return err
		}
		defer func() {
			if err := client.Disconnect(ctx); err != nil {
				log.Printf("Failed to disconnect client: %v", err)
			}
		}()

		// Ping to verify connection
		return client.Ping(ctx, nil)
	}); err != nil {
		return nil, fmt.Errorf("could not connect to docker: %w", err)
	}

	return &MongoDBContainer{
		pool:     pool,
		resource: resource,
		config:   dbConfig,
	}, nil
}

// Config returns the database configuration for the container
func (m *MongoDBContainer) Config() config.DbConfig {
	return *m.config
}

// Cleanup stops and removes the container
func (m *MongoDBContainer) Cleanup() error {
	if err := m.pool.Purge(m.resource); err != nil {
		log.Printf("Failed to purge resource: %v", err)
		return err
	}
	return nil
}

// ResetDatabase truncates all collections in the database
func (m *MongoDBContainer) ResetDatabase(ctx context.Context) error {
	client, err := mongo.Connect(
		ctx,
		options.Client().ApplyURI(m.config.Address).SetAuth(options.Credential{
			Username: m.config.Username,
			Password: m.config.Password,
		}),
	)
	if err != nil {
		return err
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("Failed to disconnect client: %v", err)
		}
	}()

	database := client.Database(m.config.DbName)
	collections, err := database.ListCollectionNames(ctx, struct{}{})
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

// Add a new method to the MongoDBContainer to get the MongoDB client
func (m *MongoDBContainer) GetClient(ctx context.Context) (*mongo.Client, error) {
	// Create a new client using the container's config
	client, err := mongo.Connect(
		ctx,
		options.Client().ApplyURI(m.config.Address).SetAuth(options.Credential{
			Username: m.config.Username,
			Password: m.config.Password,
		}),
	)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// Add a method to get the database name
func (m *MongoDBContainer) GetDatabaseName() string {
	return m.config.DbName
}
