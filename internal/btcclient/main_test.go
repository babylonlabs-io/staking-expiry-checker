package btcclient

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/ory/dockertest"
	dc "github.com/ory/dockertest/docker"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/davecgh/go-spew/spew"
)

var cfg *config.BTCConfig

func TestMain(t *testing.M) {
	var (
		cleanup func()
		err     error
	)
	cfg, cleanup, err = setupBtcd()
	if err != nil {
		log.Fatalf("Failed to setup container %v", err)
	}

	spew.Dump(cfg)

	code := t.Run()
	cleanup()
	os.Exit(code)
}

func setupBtcd() (*config.BTCConfig, func(), error) {
	const (
		rpcUser     = "user"
		rpcPassword = "password"
		// this version corresponds to docker tag for btcd
		// it should be in sync with btcd version used in production
		// todo replace with actual tag we use in prod
		btcdVersion = "latest"
		network     = "simnet"
	)

	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, nil, err
	}

	// generate random string for container name
	randomString, err := "sldkfjsldkfjlk", nil
	if err != nil {
		return nil, nil, err
	}

	// there can be only 1 container with the same name, so we add
	// random string in the end in case there is still old container running
	containerName := "btcd-integration-tests-db-" + randomString
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       containerName,
		Repository: "lnzap/btcd",
		Tag:        btcdVersion,
		Cmd: []string{
			"--" + network,
			"--rpcuser=" + rpcUser,
			"--rpcpass=" + rpcPassword,
		},
	}, func(config *dc.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = dc.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		err := pool.Purge(resource)
		if err != nil {
			// todo change to log fatal
			panic(err)
		}
	}

	// get host port (randomly chosen) that is mapped to mongo port inside container
	hostPort := resource.GetPort("8333/tcp")
	fmt.Println("Port is ", hostPort)
	cfg := &config.BTCConfig{
		RPCHost:   fmt.Sprintf("localhost:%s", hostPort),
		RPCUser:   rpcUser,
		RPCPass:   rpcPassword,
		NetParams: network,
	}

	return cfg, cleanup, nil
}
