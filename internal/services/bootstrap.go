package services

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
	"github.com/rs/zerolog/log"
)

func (s *Service) checkUnbondingOutputsForSpends(ctx context.Context) {
	log.Info().Msg("Starting check for spent unbonding outputs...")
	var (
		pageToken                  = ""
		totalUnbondedDelegations   = 0
		totalSpentUnbondingOutputs = 0
	)

	for {
		result, err := s.db.GetBTCDelegationsByStates(
			ctx,
			[]types.DelegationState{
				types.Unbonded,
			},
			pageToken,
		)
		if err != nil {
			log.Error().
				Err(err).
				Msg("Failed to fetch unbonded delegations from database")
			return
		}

		totalUnbondedDelegations += len(result.Data)

		for _, delegation := range result.Data {
			if delegation.UnbondingTx == nil {
				log.Debug().
					Str("staking_tx", delegation.StakingTxHashHex).
					Msg("Skipping delegation - no unbonding transaction found")
				continue
			}

			unbondingTx, err := utils.DeserializeBtcTransactionFromHex(delegation.UnbondingTx.TxHex)
			if err != nil {
				log.Error().
					Err(err).
					Str("unbonding_tx", delegation.UnbondingTx.TxHex).
					Msg("Failed to decode unbonding transaction")
				continue
			}
			unbondingTxHashHex := unbondingTx.TxHash().String()

			isSpent, err := s.btc.IsUTXOSpent(unbondingTxHashHex, uint32(delegation.UnbondingTx.OutputIndex))
			if err != nil {
				log.Error().
					Err(err).
					Str("staking_tx", delegation.StakingTxHashHex).
					Str("unbonding_tx", unbondingTxHashHex).
					Msg("Failed to check unbonding output spent status")
				continue
			}
			if isSpent {
				log.Info().
					Str("staking_tx", delegation.StakingTxHashHex).
					Str("unbonding_tx", unbondingTxHashHex).
					Msg("Found spent unbonding output - triggering withdrawn event")

				withdrawnEvent := types.NewWithdrawnDelegationEvent(delegation.StakingTxHashHex)
				utils.PushOrQuit(s.withdrawnDelegationChan, withdrawnEvent, s.quit)
				totalSpentUnbondingOutputs++
			}
		}

		if result.PaginationToken == "" {
			break
		}
		pageToken = result.PaginationToken
	}

	log.Info().
		Int("total_unbonded_delegations", totalUnbondedDelegations).
		Int("total_spent_unbonding_outputs", totalSpentUnbondingOutputs).
		Msg("Completed check for spent unbonding outputs")
}
