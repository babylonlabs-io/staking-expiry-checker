package utils

import (
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
)

// List of states to be ignored for unbonding as it means it's already been processed
func OutdatedStatesForUnbonding() []types.DelegationState {
	return []types.DelegationState{types.Unbonding, types.Unbonded, types.Withdrawn}
}

func OutdatedStatesForWithdraw() []types.DelegationState {
	return []types.DelegationState{types.Withdrawn}
}

// QualifiedStatesToUnbonding returns the qualified exisitng states to transition to "unbonding"
// The Active state is allowed to directly transition to Unbonding without the need of UnbondingRequested due to bootstrap usecase
func QualifiedStatesToUnbonding() []types.DelegationState {
	return []types.DelegationState{types.Active, types.UnbondingRequested}
}

// QualifiedStatesToUnbonded returns the qualified exisitng states to transition to "unbonded"
func QualifiedStatesToUnbonded(unbondTxType types.StakingTxType) []types.DelegationState {
	switch unbondTxType {
	case types.ActiveTxType:
		return []types.DelegationState{types.Active}
	case types.UnbondingTxType:
		return []types.DelegationState{types.Unbonding}
	default:
		return nil
	}
}

// QualifiedStatesToWithdrawn returns the qualified exisitng states to transition to "withdrawn"
func QualifiedStatesToWithdraw() []types.DelegationState {
	return []types.DelegationState{types.Unbonded}
}
