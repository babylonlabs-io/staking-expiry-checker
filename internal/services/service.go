package services

import (
	"sync"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/btcclient"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
	notifier "github.com/lightningnetwork/lnd/chainntnfs"
)

type Service struct {
	wg   sync.WaitGroup
	quit chan struct{}

	cfg         *config.Config
	db          db.DbInterface
	btcNotifier notifier.ChainNotifier
	btc         btcclient.BtcInterface
	trackedSubs *TrackedSubscriptions
}

func NewService(
	cfg *config.Config,
	db db.DbInterface,
	btcNotifier notifier.ChainNotifier,
	btc btcclient.BtcInterface,
) *Service {
	return &Service{
		quit:        make(chan struct{}),
		cfg:         cfg,
		db:          db,
		btcNotifier: btcNotifier,
		btc:         btc,
		trackedSubs: NewTrackedSubscriptions(),
	}
}
