package services

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
	"github.com/rs/zerolog/log"
)

// HandleUnbondingDelegationChannel processes unbonding delegations
func (s *Service) HandleUnbondingDelegationChannel(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case delegation := <-s.unbondingDelegationChan:
			log.Debug().
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("processing unbonding delegation")

			if utils.Contains(utils.OutdatedStatesForUnbonding(), delegation.State) {
				debugMsg := "delegation state is outdated for unbonding event"
				log.Ctx(ctx).Debug().Str("stakingTxHashHex", delegation.StakingTxHashHex).
					Msg(debugMsg)
				continue
			}

			if !utils.Contains(utils.QualifiedStatesToUnbonding(), delegation.State) {
				debugMsg := "delegation is not in the qualified state to transition to unbonding"
				log.Ctx(ctx).Debug().Str("stakingTxHashHex", delegation.StakingTxHashHex).
					Str("state", delegation.State.ToString()).Msg(debugMsg)
				continue
			}

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
	defer s.wg.Done()
	for {
		select {
		case delegation := <-s.withdrawnDelegationChan:
			log.Debug().
				Str("staking_tx", delegation.StakingTxHashHex).
				Msg("processing withdrawn delegation")

			if utils.Contains(utils.OutdatedStatesForWithdraw(), delegation.State) {
				debugMsg := "delegation state is outdated for withdrawn event"
				log.Ctx(ctx).Debug().Str("stakingTxHashHex", delegation.StakingTxHashHex).
					Msg(debugMsg)
				continue
			}

			if !utils.Contains(utils.QualifiedStatesToWithdraw(), delegation.State) {
				debugMsg := "delegation is not in the qualified state to transition to withdrawn"
				log.Ctx(ctx).Debug().Str("stakingTxHashHex", delegation.StakingTxHashHex).
					Str("state", delegation.State.ToString()).Msg(debugMsg)
				continue
			}

			transitionErr := s.db.TransitionToWithdrawnState(
				ctx, delegation.StakingTxHashHex,
			)
			if transitionErr != nil {
				log.Error().
					Err(transitionErr).
					Str("staking_tx", delegation.StakingTxHashHex).
					Msg("failed to transition to withdrawn state")
			}

		case <-ctx.Done():
			log.Info().Msg("stopping withdrawn channel listener: context cancelled")
			return

		case <-s.quit:
			log.Info().Msg("stopping withdrawn channel listener: service shutting down")
			return
		}
	}
}
