package services

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
	"github.com/rs/zerolog/log"
)

// handleUnbondingDelegation processes unbonding delegations
func (s *Service) handleUnbondingDelegation(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()
	for {
		select {
		case event := <-s.unbondingDelegationChan:
			log.Debug().
				Str("staking_tx", event.StakingTxHashHex).
				Msg("processing unbonding delegation")

			delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHashHex)
			if err != nil {
				log.Error().
					Err(err).
					Str("staking_tx", event.StakingTxHashHex).
					Msg("failed to get delegation")
				continue
			}

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

			expireCheckErr := s.SaveNewTimeLockExpire(ctx,
				delegation.StakingTxHashHex,
				event.UnbondingStartHeight,
				event.UnbondingTimeLock,
				types.UnbondingTxType,
			)
			if expireCheckErr != nil {
				log.Error().Err(expireCheckErr).
					Str("staking_tx", delegation.StakingTxHashHex).
					Msg("failed to process expire check")
				continue
			}

			transitionErr := s.db.TransitionToUnbondingState(
				ctx, delegation.StakingTxHashHex,
				event.UnbondingStartHeight, event.UnbondingTimeLock, event.UnbondingOutputIndex,
				event.UnbondingTxHex, event.UnbondingStartTimestamp,
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

// handleWithdrawnDelegation processes withdrawn delegations
func (s *Service) handleWithdrawnDelegation(ctx context.Context) {
	s.wg.Add(1)
	defer s.wg.Done()
	for {
		select {
		case event := <-s.withdrawnDelegationChan:
			log.Debug().
				Str("staking_tx", event.StakingTxHashHex).
				Msg("processing withdrawn delegation")

			delegation, err := s.db.GetBTCDelegationByStakingTxHash(ctx, event.StakingTxHashHex)
			if err != nil {
				log.Error().
					Err(err).
					Str("staking_tx", event.StakingTxHashHex).
					Msg("failed to get delegation")
				continue
			}

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
				utils.QualifiedStatesToWithdraw(),
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

// SaveNewTimeLockExpire checks if the staking delegation has expired and updates the database.
// This method tolerate duplicated calls on the same stakingTxHashHex.
func (s *Service) SaveNewTimeLockExpire(
	ctx context.Context, stakingTxHashHex string,
	startHeight, timelock uint64, txType types.StakingTxType,
) error {
	expireHeight := startHeight + timelock
	err := s.db.SaveTimeLockExpireCheck(
		ctx, stakingTxHashHex, expireHeight, txType.ToString(),
	)
	if err != nil {
		log.Ctx(ctx).Err(err).Msg("Failed to save expire check")
		return err
	}
	return nil
}
