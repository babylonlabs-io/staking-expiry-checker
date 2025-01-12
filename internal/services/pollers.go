package services

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
	"github.com/rs/zerolog/log"
)

func (s *Service) processBTCSubscriber(ctx context.Context) error {
	var (
		pageToken       = ""
		totalProcessed  = 0
		totalSubscribed = 0
	)
	for {
		result, err := s.db.GetBTCDelegationsByStates(
			ctx,
			[]types.DelegationState{
				types.Unbonded,
				types.UnbondingRequested,
			},
			pageToken,
		)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get delegations for BTC subscription")
			return err
		}

		totalProcessed += len(result.Data)

		// Process batch
		for _, delegation := range result.Data {
			if s.trackedSubs.IsSubscribed(delegation.StakingTxHashHex) {
				continue
			}

			if err := s.registerStakingSpendNotification(
				delegation.StakingTxHashHex,
				delegation.StakingTx.TxHex,
				uint32(delegation.StakingTx.OutputIndex),
				uint32(delegation.StakingTx.StartHeight),
			); err != nil {
				log.Error().
					Err(err).
					Str("stakingTxHash", delegation.StakingTxHashHex).
					Msg("Failed to register staking spend notification")
				return err
			}

			s.trackedSubs.AddSubscription(delegation.StakingTxHashHex)
			totalSubscribed++

			log.Debug().
				Str("stakingTxHash", delegation.StakingTxHashHex).
				Msg("Successfully registered BTC notification")
		}

		pageToken = result.PaginationToken
		if pageToken == "" {
			break
		}
	}

	log.Info().
		Int("total_processed", totalProcessed).
		Int("total_subscribed", totalSubscribed).
		Msg("BTC subscription processing completed")

	return nil
}

func (s *Service) processExpiredDelegations(ctx context.Context) error {
	btcTip, err := s.btc.GetBlockCount()
	if err != nil {
		log.Error().Err(err).Msg("Error getting BTC tip height")
		return err
	}

	// Process a single batch of expired delegations without pagination.
	// Since we delete each delegation after processing it, pagination is not needed.
	expiredDelegations, err := s.db.FindExpiredDelegations(ctx, uint64(btcTip))
	if err != nil {
		log.Error().Err(err).Msg("Error finding expired delegations")
		return err
	}

	// Process each delegation in the batch
	for _, delegation := range expiredDelegations {
		txType, err := types.StakingTxTypeFromString(delegation.TxType)
		if err != nil {
			log.Error().
				Err(err).
				Str("txType", delegation.TxType).
				Msg("Invalid timelock type")
			return err
		}

		if err := s.TransitionToUnbondedState(ctx, txType, delegation.StakingTxHashHex); err != nil {
			log.Error().
				Err(err).
				Str("stakingTxHashHex", delegation.StakingTxHashHex).
				Msg("Error transitioning delegation to unbonded")
			return err
		}

		if err := s.db.DeleteExpiredDelegation(ctx, delegation.ID); err != nil {
			log.Error().Err(err).Msg("Error deleting expired delegation")
			return err
		}
	}

	return nil
}

// TransitionToUnbondedState transitions the staking delegation to unbonded state.
// It returns true if the delegation is found and successfully transitioned to unbonded state.
func (s *Service) TransitionToUnbondedState(
	ctx context.Context, stakingTxType types.StakingTxType, stakingTxHashHex string,
) error {
	// Try to transition to unbonded, will skip if not eligible (NotFoundError)
	err := s.db.TransitionToUnbondedState(ctx, stakingTxHashHex, utils.QualifiedStatesToUnbonded(stakingTxType))
	if err != nil {
		// If the delegation is not found, we can ignore the error, it just means the delegation is not in a state that we can transition to unbonded
		if db.IsNotFoundError(err) {
			errMsg := "delegation not found or no longer eligible to be unbonded after timelock expired"
			log.Error().
				Err(err).
				Str("stakingTxHashHex", stakingTxHashHex).
				Msg(errMsg)
			return nil
		}
		log.Error().
			Err(err).
			Str("stakingTxHash", stakingTxHashHex).
			Msg("Failed to transition to unbonded state")
		return err
	}
	return nil
}
