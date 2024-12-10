package main

import (
	"context"
	"fmt"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/staking-expiry-checker/cmd/staking-expiry-checker/cli"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/btcclient"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/observability/metrics"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/poller"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/services"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Debug().Msg("failed to load .env file")
	}
}

func main() {
	ctx := context.Background()

	// setup cli commands and flags
	if err := cli.Setup(); err != nil {
		log.Fatal().Err(err).Msg("error while setting up cli")
	}

	// load config
	cfgPath := cli.GetConfigPath()
	cfg, err := config.New(cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg(fmt.Sprintf("error while loading config file: %s", cfgPath))
	}

	// initialize metrics with the metrics port from config
	metricsPort := cfg.Metrics.GetMetricsPort()
	metrics.Init(metricsPort)

	// create new db client
	dbClient, err := db.New(ctx, cfg.Db)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating db client")
	}

	btcClient, err := btcclient.NewBtcClient(&cfg.Btc)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating btc client")
	}

	btcNotifier, err := btcclient.NewBTCNotifier(
		&cfg.Btc,
		&btcclient.EmptyHintCache{},
	)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating btc notifier")
	}

	service := services.NewService(cfg, dbClient, btcNotifier, btcClient)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating service")
	}

	// Even though we pass service, it's viewed only through the specific interface
	expiryPoller := poller.NewExpiryPoller(
		cfg.Pollers.ExpiryChecker,
		service, // service implements ExpiryChecker
	)

	btcSubscriberPoller := poller.NewBTCSubscriberPoller(
		cfg.Pollers.BtcSubscriber,
		service, // service implements BTCSubscriber
	)

	// Start pollers in separate goroutines
	go expiryPoller.Start(ctx)
	go btcSubscriberPoller.Start(ctx)
}
