package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/staking-expiry-checker/cmd/staking-expiry-checker/cli"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/btcclient"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/services"
	"github.com/babylonlabs-io/staking-expiry-checker/params"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Debug().Msg("failed to load .env file")
	}
}

func main() {
	// Setup CLI commands and flags
	if err := cli.Setup(); err != nil {
		log.Fatal().Err(err).Msg("error while setting up cli")
	}

	// Load config
	cfgPath := cli.GetConfigPath()
	cfg, err := config.New(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg(fmt.Sprintf("error while loading config file: %s", cfgPath))
	}

	paramsRetriever, err := params.NewGlobalParamsRetriever(cli.GetGlobalParamsPath())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize params retriever")
	}
	versionedParams := paramsRetriever.VersionedParams()

	// Create context with signal handling
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create DB client
	var dbClient db.DbInterface
	dbClient, err = db.New(ctx, &cfg.Db)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating db client")
	}
	dbClient = db.NewDbWithMetrics(dbClient)

	// Create BTC client
	btcClient, err := btcclient.NewBtcClient(&cfg.Btc)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating btc client")
	}

	// Create BTC notifier
	btcNotifier, err := btcclient.NewBTCNotifier(
		&cfg.Btc,
		&btcclient.EmptyHintCache{},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating btc notifier")
	}

	// Create service
	service := services.NewService(cfg, versionedParams, dbClient, btcNotifier, btcClient)
	if err := service.RunUntilShutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start service")
	}
}
