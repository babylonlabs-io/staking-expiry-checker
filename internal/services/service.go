package services

import (
	"sync"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/btcclient"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	notifier "github.com/lightningnetwork/lnd/chainntnfs"
)

type Service struct {
	wg   sync.WaitGroup
	quit chan struct{}

	cfg         *config.Config
	btcNotifier notifier.ChainNotifier

	// interfaces
	db  db.DbInterface
	btc btcclient.BtcInterface

	// in memory stores
	trackedSubs *TrackedSubscriptions

	// channels
	unbondingDelegationChan chan *types.UnbondingDelegationEvent
	withdrawnDelegationChan chan *types.WithdrawnDelegationEvent
}

func NewService(
	cfg *config.Config,
	db db.DbInterface,
	btcNotifier notifier.ChainNotifier,
	btc btcclient.BtcInterface,
) *Service {
	return &Service{
		quit:                    make(chan struct{}),
		cfg:                     cfg,
		db:                      db,
		btcNotifier:             btcNotifier,
		btc:                     btc,
		trackedSubs:             NewTrackedSubscriptions(),
		unbondingDelegationChan: make(chan *types.UnbondingDelegationEvent, 100), // buffered
		withdrawnDelegationChan: make(chan *types.WithdrawnDelegationEvent, 100), // buffered
	}
}
