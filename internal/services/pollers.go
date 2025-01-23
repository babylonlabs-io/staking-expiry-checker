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
				// We use a hybrid approach to monitor unbonding output spends:
				// 1. First check if the output is already spent using direct RPC call
				// 2. Only register for spend notifications if the output is still unspent
				//
				// This avoids putting unnecessary load on the BTC notifier service which needs
				// to maintain subscriptions and is not optimized for historical transaction scanning.
				// The RPC call is more efficient for checking historical spend status.
				unbondingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.UnbondingTx.TxHex)
				if err != nil {
					return fmt.Errorf("failed to decode unbonding transaction: %w", err)
				}

				unbondingTxHashHex := unbondingTx.TxHash().String()
				isSpent, err := s.btc.IsUTXOSpent(unbondingTxHashHex, uint32(delegation.UnbondingTx.OutputIndex))
				if err != nil {
					return fmt.Errorf("failed to check unbonding output spent status: %w", err)
				}
				if isSpent {
					// Output is spent - trigger withdrawn event
					log.Info().
						Str("staking_tx", delegation.StakingTxHashHex).
						Str("unbonding_tx", unbondingTxHashHex).
						Msg("Found spent unbonding output - triggering withdrawn event")

					// TODO: We should validate the spending tx using validateWithdrawalTxFromUnbonding before
					// considering it withdrawn. However, Bitcoin RPC doesn't provide spending tx details directly.
					// The only way would be historical block scanning which is inefficient. For now we skip
					// validation, but we have a few options to improve this:
					// 1. Implement efficient historical block scanning to find the spending tx
					// 2. Run phase 1 indexer stack to identify any discrepancies in withdrawal txs

					withdrawnEvent := types.NewWithdrawnDelegationEvent(delegation.StakingTxHashHex)
					utils.PushOrQuit(s.withdrawnDelegationChan, withdrawnEvent, s.quit)
				} else {
					// Output not spent - register for spend notifications
					if err := s.registerUnbondingSpendNotification(
						delegation.StakingTxHashHex,
						delegation.UnbondingTx.TxHex,
						uint32(delegation.UnbondingTx.StartHeight),
					); err != nil {
						log.Error().
							Err(err).
							Str("stakingTxHash", delegation.StakingTxHashHex).
							Msg("Failed to register unbonding spend notification")
						return fmt.Errorf("failed to register unbonding spend notification: %w", err)
					}

					s.trackedSubs.AddSubscription(delegation.StakingTxHashHex)
					totalSubscribed++
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
				s.trackedSubs.AddSubscription(delegation.StakingTxHashHex)
				totalSubscribed++
			}

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
