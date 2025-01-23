package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/babylonlabs-io/networks/parameters/parser"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/btcclient"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/observability/metrics"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	notifier "github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/rs/zerolog/log"
)

type Service struct {
	wg   sync.WaitGroup
	quit chan struct{}

	cfg            *config.Config
	btcNotifier    notifier.ChainNotifier
	paramsVersions *parser.ParsedGlobalParams

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
	paramsVersions *parser.ParsedGlobalParams,
	db db.DbInterface,
	btcNotifier notifier.ChainNotifier,
	btc btcclient.BtcInterface,
) *Service {
	return &Service{
		quit:                    make(chan struct{}),
		cfg:                     cfg,
		btcNotifier:             btcNotifier,
		paramsVersions:          paramsVersions,
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
	defer func() {
		if err := s.btcNotifier.Stop(); err != nil {
			log.Error().Err(err).Msg("failed to stop btc chain notifier")
		}
	}()

	// Start pollers
	go s.startExpiryPoller(ctx)
	go s.startBTCSubscriberPoller(ctx)

	// Start service handlers
	go s.handleUnbondingDelegation(ctx)
	go s.handleWithdrawnDelegation(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	log.Info().Msg("Shutdown signal received, stopping service...")

	// Signal all components to stop
	close(s.quit)

	// Wait for all goroutines to finish
	s.wg.Wait()

	return nil
}

func (s *Service) startExpiryPoller(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.Pollers.ExpiryChecker.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pollingCtx, cancel := context.WithTimeout(ctx, s.cfg.Pollers.ExpiryChecker.Timeout)
			start := time.Now()
			log.Debug().Msg("starting expiry poller")
			err := s.processExpiredDelegations(pollingCtx)
			if err != nil {
				log.Error().Err(err).Msg("Error processing expired delegations")
			}
			duration := time.Since(start)
			metrics.ObservePollerDuration("expiry_poller", duration, err)
			cancel()
		case <-ctx.Done():
			log.Info().Msg("Expiry poller stopped due to context cancellation")
			return
		case <-s.quit:
			return
		}
	}
}

func (s *Service) startBTCSubscriberPoller(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.Pollers.BtcSubscriber.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pollingCtx, cancel := context.WithTimeout(ctx, s.cfg.Pollers.BtcSubscriber.Timeout)
			start := time.Now()
			log.Debug().Msg("starting BTC subscriber poller")
			err := s.processBTCSubscriber(pollingCtx)
			if err != nil {
				log.Error().Err(err).Msg("Error processing BTC subscriptions")
			}
			duration := time.Since(start)
			metrics.ObservePollerDuration("btc_subscriber_poller", duration, err)
			cancel()
		case <-ctx.Done():
			log.Info().Msg("BTC subscriber poller stopped due to context cancellation")
			return
		case <-s.quit:
			return
		}
	}
}

func (s *Service) getVersionedParams(height uint64) (*parser.ParsedVersionedGlobalParams, error) {
	params := s.paramsVersions.GetVersionedGlobalParamsByHeight(height)
	if params == nil {
		return nil, fmt.Errorf("the params for height %d does not exist", height)
	}

	return params, nil
}
