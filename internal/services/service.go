package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/btcclient"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/observability/metrics"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/poller"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	notifier "github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/rs/zerolog/log"
)

type Service struct {
	wg   sync.WaitGroup
	quit chan struct{}

	cfg         *config.Config
	btcNotifier notifier.ChainNotifier
	params      *types.GlobalParams

	// interfaces
	db  db.DbInterface
	btc btcclient.BtcInterface

	// in memory stores
	trackedSubs *TrackedSubscriptions

	// channels
	unbondingDelegationChan chan *types.UnbondingDelegationEvent
	withdrawnDelegationChan chan *types.WithdrawnDelegationEvent
}

func NewService(
	cfg *config.Config,
	params *types.GlobalParams,
	db db.DbInterface,
	btcNotifier notifier.ChainNotifier,
	btc btcclient.BtcInterface,
) *Service {
	return &Service{
		quit:                    make(chan struct{}),
		cfg:                     cfg,
		btcNotifier:             btcNotifier,
		params:                  params,
		db:                      db,
		btc:                     btc,
		trackedSubs:             NewTrackedSubscriptions(),
		unbondingDelegationChan: make(chan *types.UnbondingDelegationEvent, 100), // buffered
		withdrawnDelegationChan: make(chan *types.WithdrawnDelegationEvent, 100), // buffered
	}
}

func (s *Service) RunUntilShutdown(ctx context.Context) error {
	// Initialize metrics
	metricsPort := s.cfg.Metrics.GetMetricsPort()
	metrics.Init(metricsPort)

	// Start BTCNotifier
	if err := s.btcNotifier.Start(); err != nil {
		return fmt.Errorf("failed to start btc chain notifier: %w", err)
	}
	defer s.btcNotifier.Stop()

	// Start pollers
	go poller.NewExpiryPoller(s.cfg.Pollers.ExpiryChecker, s).Start(ctx)
	go poller.NewBTCSubscriberPoller(s.cfg.Pollers.BtcSubscriber, s).Start(ctx)

	// Start service handlers
	go s.HandleUnbondingDelegationChannel(ctx)
	go s.HandleWithdrawnDelegationChannel(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	log.Info().Msg("Shutdown signal received, stopping service...")

	return nil
}
