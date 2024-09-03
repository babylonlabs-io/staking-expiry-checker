package services

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/btcclient"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/queue"
	queueclient "github.com/babylonlabs-io/staking-queue-client/client"
	"github.com/rs/zerolog/log"
)

type Service struct {
	db           db.DbInterface
	btc          btcclient.BtcInterface
	queueManager *queue.QueueManager
}

func NewService(db db.DbInterface, btc btcclient.BtcInterface, qm *queue.QueueManager) *Service {
	return &Service{
		db:           db,
		btc:          btc,
		queueManager: qm,
	}
}

func (s *Service) ProcessExpiredDelegations(ctx context.Context) error {
	// TODO: Use cache with ttl to store the tip height.
	btcTip, err := s.btc.GetBlockCount()
	if err != nil {
		log.Error().Err(err).Msg("Error getting BTC tip height")
		return err
	}

	for {
		expiredDelegations, err := s.db.FindExpiredDelegations(ctx, uint64(btcTip))
		if err != nil {
			log.Error().Err(err).Msg("Error finding expired delegations")
			return err
		}
		if len(expiredDelegations) == 0 {
			break
		}

		for _, delegation := range expiredDelegations {
			ev := queueclient.NewExpiredStakingEvent(delegation.StakingTxHashHex, delegation.TxType)
			if err := s.queueManager.SendExpiredStakingEvent(ctx, ev); err != nil {
				log.Error().Err(err).Msg("Error sending expired staking event")
				return err
			}
			// After successfully sending the event, delete the entry from the database.
			if err := s.db.DeleteExpiredDelegation(ctx, delegation.ID); err != nil {
				log.Error().Err(err).Msg("Error deleting expired delegation")
				return err
			}
		}
	}

	return nil
}
