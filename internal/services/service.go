package services

import (
	"context"
	"sync"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/btcclient"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
	notifier "github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/rs/zerolog/log"
)

type Service struct {
	wg   sync.WaitGroup
	quit chan struct{}

	cfg         *config.Config
	db          db.DbInterface
	btcNotifier notifier.ChainNotifier
	btc         btcclient.BtcInterface
	trackedSubs *TrackedSubscriptions

	unbondingDelegationChan chan *model.BTCDelegationDetails
	withdrawnDelegationChan chan *model.BTCDelegationDetails
}

func NewService(
	cfg *config.Config,
	db db.DbInterface,
	btcNotifier notifier.ChainNotifier,
	btc btcclient.BtcInterface,
) *Service {
	return &Service{
		quit:                    make(chan struct{}),
		cfg:                     cfg,
		db:                      db,
		btcNotifier:             btcNotifier,
		btc:                     btc,
		trackedSubs:             NewTrackedSubscriptions(),
		unbondingDelegationChan: make(chan *model.BTCDelegationDetails, 100), // buffered
		withdrawnDelegationChan: make(chan *model.BTCDelegationDetails, 100), // buffered
	}
}

// HandleUnbondingDelegationChannel processes unbonding delegations
func (s *Service) HandleUnbondingDelegationChannel(ctx context.Context) {
	for {
		select {
		case delegation := <-s.unbondingDelegationChan:
			log.Debug().
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("processing unbonding delegation")

			if utils.Contains(utils.OutdatedStatesForUnbonding(), delegation.State) {
				// Ignore the message as the delegation state already passed the unbonding state. This is an outdated duplication
				log.Ctx(ctx).Debug().Str("StakingTxHashHex", delegation.StakingTxHashHex).
					Msg("delegation state is outdated for unbonding event")
				continue
			}

			// Save the unbonding staking delegation. This is the final step in the unbonding staking event processing
			// Please refer to the README.md for the details on the unbonding staking event processing workflow
			transitionErr := s.db.TransitionToUnbondingState(
				ctx, delegation.StakingTxHashHex,
			)
			if transitionErr != nil {
				log.Error().
					Err(transitionErr).
					Str("staking_tx", delegation.StakingTxHashHex).
					Msg("failed to transition to unbonding state")
			}

		case <-ctx.Done():
			log.Info().Msg("stopping unbonding channel listener: context cancelled")
			return

		case <-s.quit:
			log.Info().Msg("stopping unbonding channel listener: service shutting down")
			return
		}
	}
}

// HandleWithdrawnDelegationChannel processes withdrawn delegations
func (s *Service) HandleWithdrawnDelegationChannel(ctx context.Context) {
	for {
		select {
		case delegation := <-s.withdrawnDelegationChan:
			log.Debug().
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("processing withdrawn delegation")

			// if err := s.processWithdrawnDelegation(ctx, delegation); err != nil {
			// 	log.Error().
			// 		Err(err).
			// 		Str("staking_tx", delegation.StakingTxHashHex).
			// 		Msg("failed to process withdrawn delegation")
			// }

		case <-ctx.Done():
			log.Info().Msg("stopping withdrawn channel listener: context cancelled")
			return

		case <-s.quit:
			log.Info().Msg("stopping withdrawn channel listener: service shutting down")
			return
		}
	}
}
