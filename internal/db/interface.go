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
	TransitionToUnbondedState(
		ctx context.Context,
		stakingTxHashHex string,
		unbondTxType types.StakingTxType,
	) error
	TransitionToUnbondingState(
		ctx context.Context,
		stakingTxHashHex string,
	) error
	TransitionToWithdrawnState(
		ctx context.Context,
		stakingTxHashHex string,
	) error
	GetBTCDelegationByStakingTxHash(
		ctx context.Context, stakingTxHash string,
	) (*model.DelegationDocument, error)
	GetStakingParams(ctx context.Context, version uint32) (*model.StakingParams, error)
	GetBTCDelegationsByStates(ctx context.Context, states []types.DelegationState) ([]*model.DelegationDocument, error)
	GetBTCDelegationState(ctx context.Context, stakingTxHash string) (*types.DelegationState, error)
}
