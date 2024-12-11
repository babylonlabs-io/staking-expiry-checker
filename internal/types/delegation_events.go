package types

type UnbondingDelegationEvent struct {
	StakingTxHashHex     string
	UnbondingStartHeight uint32
}

type WithdrawnDelegationEvent struct {
	StakingTxHashHex string
}

func NewUnbondingDelegationEvent(stakingTxHashHex string, unbondingStartHeight uint32) *UnbondingDelegationEvent {
	return &UnbondingDelegationEvent{
		StakingTxHashHex:     stakingTxHashHex,
		UnbondingStartHeight: unbondingStartHeight,
	}
}

func NewWithdrawnDelegationEvent(stakingTxHashHex string) *WithdrawnDelegationEvent {
	return &WithdrawnDelegationEvent{
		StakingTxHashHex: stakingTxHashHex,
	}
}
