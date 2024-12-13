package types

type UnbondingDelegationEvent struct {
	StakingTxHashHex        string
	UnbondingStartHeight    uint64
	UnbondingStartTimestamp int64
	UnbondingTimeLock       uint64
	UnbondingOutputIndex    uint64
	UnbondingTxHex          string
	UnbondingTxHashHex      string
}

type WithdrawnDelegationEvent struct {
	StakingTxHashHex string
}

func NewUnbondingDelegationEvent(
	stakingTxHashHex string,
	unbondingStartHeight uint64,
	unbondingStartTimestamp int64,
	unbondingTimeLock uint64,
	unbondingOutputIndex uint64,
	unbondingTxHex string,
	unbondingTxHashHex string,
) *UnbondingDelegationEvent {
	return &UnbondingDelegationEvent{
		StakingTxHashHex:        stakingTxHashHex,
		UnbondingStartHeight:    unbondingStartHeight,
		UnbondingStartTimestamp: unbondingStartTimestamp,
		UnbondingTimeLock:       unbondingTimeLock,
		UnbondingOutputIndex:    unbondingOutputIndex,
		UnbondingTxHex:          unbondingTxHex,
		UnbondingTxHashHex:      unbondingTxHashHex,
	}
}

func NewWithdrawnDelegationEvent(stakingTxHashHex string) *WithdrawnDelegationEvent {
	return &WithdrawnDelegationEvent{
		StakingTxHashHex: stakingTxHashHex,
	}
}
