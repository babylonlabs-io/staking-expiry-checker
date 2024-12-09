package services

import (
	"github.com/babylonlabs-io/staking-expiry-checker/internal/btcclient"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db"
)

type Service struct {
	db  db.DbInterface
	btc btcclient.BtcInterface
}

func NewService(db db.DbInterface, btc btcclient.BtcInterface) *Service {
	return &Service{
		db:  db,
		btc: btc,
	}
}
