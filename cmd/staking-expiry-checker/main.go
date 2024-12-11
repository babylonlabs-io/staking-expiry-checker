package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"

	"github.com/babylonlabs-io/staking-expiry-checker/cmd/staking-expiry-checker/cli"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/btcclient"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/observability/metrics"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/poller"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/services"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Debug().Msg("failed to load .env file")
	}
}

func main() {
	// Create a context that is cancelled on SIGINT or SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

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

	paramsPath := cli.GetGlobalParamsPath()
	params, err := types.NewGlobalParams(paramsPath)
	if err != nil {
		log.Fatal().Err(err).Msg(fmt.Sprintf("error while loading global params file: %s", paramsPath))
	}

	// Initialize metrics with the metrics port from config
	metricsPort := cfg.Metrics.GetMetricsPort()
	metrics.Init(metricsPort)

	// Create new DB client
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

	service := services.NewService(cfg, params, dbClient, btcNotifier, btcClient)
	if err != nil {
		log.Fatal().Err(err).Msg("error while creating service")
	}

	// Start pollers
	go poller.NewExpiryPoller(cfg.Pollers.ExpiryChecker, service).Start(ctx)
	go poller.NewBTCSubscriberPoller(cfg.Pollers.BtcSubscriber, service).Start(ctx)

	// Start service handlers
	go service.HandleUnbondingDelegationChannel(ctx)
	go service.HandleWithdrawnDelegationChannel(ctx)

	// Wait for a signal to shutdown
	<-sigChan

	// Cancel the context to signal all goroutines to stop
	cancel()
}
