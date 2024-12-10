package db

import (
	"context"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DbInterface interface {
	Ping(ctx context.Context) error
	FindExpiredDelegations(
		ctx context.Context, btcTipHeight uint64,
	) ([]model.TimeLockDocument, error)
	DeleteExpiredDelegation(
		ctx context.Context, id primitive.ObjectID,
	) error
	SaveTimeLockExpireCheck(
		ctx context.Context, stakingTxHashHex string,
		expireHeight uint64, txType string,
	) error
	TransitionToUnbonded(
		ctx context.Context,
		stakingTxHashHex string,
		unbondTxType types.TransactionType,
	) error
	GetBTCDelegationByStakingTxHash(
		ctx context.Context, stakingTxHash string,
	) (*model.BTCDelegationDetails, error)
	GetStakingParams(ctx context.Context, version uint32) (*model.StakingParams, error)
	GetBTCDelegationsByStates(ctx context.Context, states []model.DelegationState) ([]*model.BTCDelegationDetails, error)
}
