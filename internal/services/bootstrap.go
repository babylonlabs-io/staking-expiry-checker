package services

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
	"github.com/rs/zerolog/log"
)

func (s *Service) checkUnbondingOutputsForSpends(ctx context.Context) {
	log.Info().Msg("Starting check for spent unbonding outputs...")
	var pageToken = ""

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

		for _, delegation := range result.Data {
			if delegation.UnbondingTx == nil {
				log.Debug().
					Str("staking_tx", delegation.StakingTxHashHex).
					Msg("Skipping delegation - no unbonding transaction found")
				continue
			}

			isSpent, err := s.btc.IsUTXOSpent(delegation.UnbondingTx.TxHex, uint32(delegation.UnbondingTx.OutputIndex))
			if err != nil {
				log.Error().
					Err(err).
					Str("staking_tx", delegation.StakingTxHashHex).
					Str("unbonding_tx", delegation.UnbondingTx.TxHex).
					Uint32("output_index", uint32(delegation.UnbondingTx.OutputIndex)).
					Msg("Failed to check unbonding output spent status")
				continue
			}
			if isSpent {
				log.Info().
					Str("staking_tx", delegation.StakingTxHashHex).
					Str("unbonding_tx", delegation.UnbondingTx.TxHex).
					Uint32("output_index", uint32(delegation.UnbondingTx.OutputIndex)).
					Msg("Found spent unbonding output - triggering withdrawn event")

				withdrawnEvent := types.NewWithdrawnDelegationEvent(delegation.StakingTxHashHex)
				utils.PushOrQuit(s.withdrawnDelegationChan, withdrawnEvent, s.quit)
			}
		}

		if result.PaginationToken == "" {
			break
		}
		pageToken = result.PaginationToken
	}

	log.Info().Msg("Completed check for spent unbonding outputs")
}
