package services

import (
	"context"
	"fmt"

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
			return fmt.Errorf("error getting BTC delegations by states: %w", err)
		}

		totalProcessed += len(result.Data)

		// Process batch
		for _, delegation := range result.Data {
			if s.trackedSubs.IsSubscribed(delegation.StakingTxHashHex) {
				continue
			}

			if delegation.State == types.Unbonded && delegation.UnbondingTx != nil {
				// For early unbonded delegations i.e. state is Unbonded and Unbonding Tx is present:
				// 1. Staking output is already spent by the unbonding tx
				// 2. Track unbonding output to detect withdrawal tx
				if err := s.registerUnbondingSpendNotification(
					delegation.StakingTxHashHex,
					delegation.UnbondingTx.TxHex,
					delegation.UnbondingTx.StartHeight,
				); err != nil {
					log.Error().
						Err(err).
						Str("stakingTxHash", delegation.StakingTxHashHex).
						Msg("Failed to register unbonding spend notification")
					return fmt.Errorf("failed to register unbonding spend notification: %w", err)
				}
			} else {
				// For all other cases, we track the staking transaction output:
				// 1. Natural unbonding: Need to detect withdrawal tx
				// 2. Unbonding requested: Need to monitor staking output
				//    until the unbonding transaction is found.
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
					return fmt.Errorf("failed to register staking spend notification: %w", err)
				}
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
		return fmt.Errorf("error getting BTC tip height: %w", err)
	}

	// Process a single batch of expired delegations without pagination.
	// Since we delete each delegation after processing it, pagination is not needed.
	expiredDelegations, err := s.db.FindExpiredDelegations(ctx, uint64(btcTip))
	if err != nil {
		return fmt.Errorf("error finding expired delegations: %w", err)
	}

	// Process each delegation in the batch
	for _, delegation := range expiredDelegations {
		txType, err := types.StakingTxTypeFromString(delegation.TxType)
		if err != nil {
			return fmt.Errorf("invalid timelock type: %w", err)
		}

		if err := s.TransitionToUnbondedState(ctx, txType, delegation.StakingTxHashHex); err != nil {
			return fmt.Errorf("error transitioning delegation to unbonded: %w", err)
		}

		if err := s.db.DeleteExpiredDelegation(ctx, delegation.ID); err != nil {
			return fmt.Errorf("error deleting expired delegation: %w", err)
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
			log.Error().
				Err(err).
				Str("stakingTxHashHex", stakingTxHashHex).
				Msg("delegation not found or no longer eligible to be unbonded after timelock expired")
			return nil
		}
		log.Error().
			Err(err).
			Str("stakingTxHash", stakingTxHashHex).
			Msg("failed to transition to unbonded state")
		return fmt.Errorf("failed to transition to unbonded state: %w", err)
	}
	return nil
}
