package services

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"github.com/rs/zerolog/log"
)

// ProcessExpireCheck checks if the staking delegation has expired and updates the database.
// This method tolerate duplicated calls on the same stakingTxHashHex.
func (s *Service) ProcessExpireCheck(
	ctx context.Context, stakingTxHashHex string,
	startHeight, timelock uint64, txType types.StakingTxType,
) *types.Error {
	expireHeight := startHeight + timelock
	err := s.db.SaveTimeLockExpireCheck(
		ctx, stakingTxHashHex, expireHeight, txType.ToString(),
	)
	if err != nil {
		log.Ctx(ctx).Err(err).Msg("Failed to save expire check")
		return types.NewInternalServiceError(err)
	}
	return nil
}

func (s *Service) ProcessExpiredDelegations(ctx context.Context) *types.Error {
	btcTip, err := s.btc.GetBlockCount()
	if err != nil {
		log.Error().Err(err).Msg("Error getting BTC tip height")
		return types.NewInternalServiceError(err)
	}

	for {
		expiredDelegations, err := s.db.FindExpiredDelegations(ctx, uint64(btcTip))
		if err != nil {
			log.Error().Err(err).Msg("Error finding expired delegations")
			return types.NewInternalServiceError(err)
		}
		if len(expiredDelegations) == 0 {
			break
		}

		for _, delegation := range expiredDelegations {
			err := s.ProcessExpiredDelegation(ctx, delegation)
			if err != nil {
				log.Error().Err(err).Msgf("Error processing expired delegation: %v", delegation.ID)
				return err
			}

			// After successfully sending the event, delete the entry from the database.
			if err := s.db.DeleteExpiredDelegation(ctx, delegation.ID); err != nil {
				log.Error().Err(err).Msg("Error deleting expired delegation")
				return types.NewInternalServiceError(err)
			}
		}
	}

	return nil
}

// ProcessExpiredDelegation processes an expired delegation by
// transitioning it to unbonded.
// Do nothing if the delegation is not in an eligible state to transition.
func (s *Service) ProcessExpiredDelegation(
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
