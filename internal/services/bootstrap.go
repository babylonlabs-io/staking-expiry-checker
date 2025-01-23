package services

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
	"github.com/rs/zerolog/log"
)

func (s *Service) checkUnbondingOutputsForSpends(ctx context.Context) {
	var (
		pageToken = ""
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
			log.Error().Err(err).Msg("error getting BTC delegations during initial UTXO check")
			return
		}

		for _, delegation := range result.Data {
			if delegation.UnbondingTx == nil {
				// we are only interested in early unbonded delegations
				// which have unbonding txs
				log.Debug().
					Str("staking_tx", delegation.StakingTxHashHex).
					Msg("unbonded delegation has no unbonding tx")
				continue
			}

			isSpent, err := s.btc.IsUTXOSpent(delegation.UnbondingTx.TxHex, uint32(delegation.UnbondingTx.OutputIndex))
			if err != nil {
				log.Error().Err(err).Msg("failed to check if UTXO is spent")
				continue
			}
			if isSpent {
				log.Info().
					Str("staking_tx", delegation.StakingTxHashHex).
					Msg("unbonding output is spent")

				withdrawnEvent := types.NewWithdrawnDelegationEvent(delegation.StakingTxHashHex)
				utils.PushOrQuit(s.withdrawnDelegationChan, withdrawnEvent, s.quit)
			}
		}

		if result.PaginationToken == "" {
			break
		}
		pageToken = result.PaginationToken
	}

	log.Info().Msg("Completed initial check for spent UTXOs")
}
