package db

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"context"
	"time"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/observability/metrics"
)

type dbWithMetrics struct {
	db DbInterface
}

func NewDbWithMetrics(db DbInterface) *dbWithMetrics {
	return &dbWithMetrics{db: db}
}

func (d *dbWithMetrics) Ping(ctx context.Context) error {
	return d.db.Ping(ctx)
}

func (d *dbWithMetrics) FindExpiredDelegations(ctx context.Context, btcTipHeight uint64) ([]model.TimeLockDocument, error) {
	return runAndMeasureLatency("FindExpiredDelegations", func() ([]model.TimeLockDocument, error) {
		return d.db.FindExpiredDelegations(ctx, btcTipHeight)
	})
}

func (d *dbWithMetrics) DeleteExpiredDelegation(ctx context.Context, id primitive.ObjectID) error {
	return d.run("DeleteExpiredDelegation", func() error {
		return d.db.DeleteExpiredDelegation(ctx, id)
	})
}

func (d *dbWithMetrics) SaveTimeLockExpireCheck(ctx context.Context, stakingTxHashHex string, expireHeight uint64, txType string) error {
	return d.run("SaveTimeLockExpireCheck", func() error {
		return d.db.SaveTimeLockExpireCheck(ctx, stakingTxHashHex, expireHeight, txType)
	})
}

func (d *dbWithMetrics) TransitionToUnbondedState(ctx context.Context, stakingTxHashHex string, eligiblePreviousStates []types.DelegationState) error {
	return d.run("TransitionToUnbondedState", func() error {
		return d.db.TransitionToUnbondedState(ctx, stakingTxHashHex, eligiblePreviousStates)
	})
}

func (d *dbWithMetrics) TransitionToUnbondingState(ctx context.Context, stakingTxHashHex string, unbondingStartHeight, unbondingTimelock, unbondingOutputIndex uint64, unbondingTxHex string, unbondingStartTimestamp int64) error {
	return d.run("TransitionToUnbondingState", func() error {
		return d.db.TransitionToUnbondingState(ctx, stakingTxHashHex, unbondingStartHeight, unbondingTimelock, unbondingOutputIndex, unbondingTxHex, unbondingStartTimestamp)
	})
}

func (d *dbWithMetrics) TransitionToWithdrawnState(ctx context.Context, stakingTxHashHex string, eligiblePreviousStates []types.DelegationState) error {
	return d.run("TransitionToWithdrawnState", func() error {
		return d.db.TransitionToWithdrawnState(ctx, stakingTxHashHex, eligiblePreviousStates)
	})
}

func (d *dbWithMetrics) GetBTCDelegationByStakingTxHash(ctx context.Context, stakingTxHash string) (*model.DelegationDocument, error) {
	return runAndMeasureLatency("GetBTCDelegationByStakingTxHash", func() (*model.DelegationDocument, error) {
		return d.db.GetBTCDelegationByStakingTxHash(ctx, stakingTxHash)
	})
}

func (d *dbWithMetrics) GetBTCDelegationsByStates(ctx context.Context, states []types.DelegationState, paginationToken string) (*DbResultMap[model.DelegationDocument], error) {
	return runAndMeasureLatency("GetBTCDelegationsByStates", func() (*DbResultMap[model.DelegationDocument], error) {
		return d.db.GetBTCDelegationsByStates(ctx, states, paginationToken)
	})
}

func (d *dbWithMetrics) GetBTCDelegationState(ctx context.Context, stakingTxHash string) (*types.DelegationState, error) {
	return runAndMeasureLatency("GetBTCDelegationState", func() (*types.DelegationState, error) {
		return d.db.GetBTCDelegationState(ctx, stakingTxHash)
	})
}

// run is method that simplifies calls to runAndMeasureLatency in cases where only error is returned
func (d *dbWithMetrics) run(method string, f func() error) error {
	// auxiliary type for functions that return only error
	type zero struct{}
	_, err := runAndMeasureLatency(method, func() (zero, error) {
		return zero{}, f()
	})

	return err
}

func runAndMeasureLatency[T any](method string, f func() (T, error)) (T, error) {
	startTime := time.Now()
	v, err := f()
	duration := time.Since(startTime)

	metrics.ObserveDBLatency(method, duration, err != nil)
	return v, err
}
