package services

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/rs/zerolog/log"
)

func (s *Service) processBTCSubscriber(ctx context.Context) *types.Error {
	// Get delegations that need BTC notifications
	delegations, err := s.db.GetBTCDelegationsByStates(ctx, []types.DelegationState{
		types.Unbonded,
		types.UnbondingRequested,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to get delegations for BTC subscription")
		return types.NewInternalServiceError(err)
	}

	if len(delegations) == 0 {
		log.Debug().Msg("No delegations found for BTC subscription")
		return nil
	}

	// Process each delegation
	for _, delegation := range delegations {
		if s.trackedSubs.IsSubscribed(delegation.StakingTxHashHex) {
			log.Debug().
				Str("stakingTxHash", delegation.StakingTxHashHex).
				Msg("Delegation already subscribed, skipping")
			continue
		}

		err := s.registerStakingSpendNotification(
			delegation.StakingTxHashHex,
			delegation.StakingTx.TxHex,
			uint32(delegation.StakingTx.OutputIndex),
			uint32(delegation.StakingTx.StartHeight),
		)
		if err != nil {
			log.Error().
				Err(err).
				Str("stakingTxHash", delegation.StakingTxHashHex).
				Msg("Failed to register staking spend notification")
			return types.NewInternalServiceError(err)
		}

		// Add to tracked subscriptions after successful registration
		s.trackedSubs.AddSubscription(delegation.StakingTxHashHex)

		log.Debug().
			Str("stakingTxHash", delegation.StakingTxHashHex).
			Msg("Successfully registered BTC notification")
	}

	return nil
}

func (s *Service) processExpiredDelegations(ctx context.Context) *types.Error {
	btcTip, err := s.btc.GetBlockCount()
	if err != nil {
		log.Error().Err(err).Msg("Error getting BTC tip height")
		return types.NewInternalServiceError(err)
	}

	// Single batch of expired delegations
	expiredDelegations, err := s.db.FindExpiredDelegations(ctx, uint64(btcTip))
	if err != nil {
		log.Error().Err(err).Msg("Error finding expired delegations")
		return types.NewInternalServiceError(err)
	}

	// Process each delegation in the batch
	for _, delegation := range expiredDelegations {
		if err := s.transitionToUnbondedIfEligible(ctx, delegation); err != nil {
			log.Error().Err(err).
				Msgf("Error transitioning delegation to unbonded: %v", delegation.ID)
			return err
		}

		if err := s.db.DeleteExpiredDelegation(ctx, delegation.ID); err != nil {
			log.Error().Err(err).Msg("Error deleting expired delegation")
			return types.NewInternalServiceError(err)
		}
	}

	return nil
}

// transitionToUnbondedIfEligible attempts to transition a delegation to unbonded state
// if it's in an eligible state.
func (s *Service) transitionToUnbondedIfEligible(
	ctx context.Context, delegation model.TimeLockDocument,
) *types.Error {
	// Check what type of the timelock is
	timelockType, err := types.StakingTxTypeFromString(delegation.TxType)
	if err != nil {
		log.Error().Err(err).Msgf("Invalid timelock type: %s", delegation.TxType)
		return types.NewInternalServiceError(err)
	}

	// Try to transition to unbonded, will skip if not eligible (NotFoundError)
	err = s.db.TransitionToUnbondedState(
		ctx, delegation.StakingTxHashHex, timelockType,
	)
	if err != nil {
		if db.IsNotFoundError(err) {
			// Silently skip if not eligible
			log.Debug().Msgf(
				"Delegation not found or not in eligible state to transition: %v", delegation.ID,
			)
			return nil
		}
		log.Error().Err(err).Msgf("Error transitioning to unbonded: %v", delegation.ID)
		return types.NewInternalServiceError(err)
	}

	return nil
}
