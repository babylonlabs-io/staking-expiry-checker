package services

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/rs/zerolog/log"
)

func (s *Service) ProcessBTCSubscriber(ctx context.Context) *types.Error {
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
			delegation.StakingTxHex,
			delegation.StakingOutputIdx,
			delegation.StartHeight,
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
