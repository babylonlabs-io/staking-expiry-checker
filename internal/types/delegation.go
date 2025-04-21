package types

type DelegationState string

const (
	Active             DelegationState = "active"
	UnbondingRequested DelegationState = "unbonding_requested"
	Unbonding          DelegationState = "unbonding"
	Unbonded           DelegationState = "unbonded"
	Withdrawn          DelegationState = "withdrawn"
	Transitioned       DelegationState = "transitioned"
)

func (s DelegationState) ToString() string {
	return string(s)
}
