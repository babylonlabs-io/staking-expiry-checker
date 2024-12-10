package model

import "fmt"

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

func FromStringToDelegationState(s string) (DelegationState, error) {
	switch s {
	case "active":
		return Active, nil
	case "unbonding_requested":
		return UnbondingRequested, nil
	case "unbonding":
		return Unbonding, nil
	case "unbonded":
		return Unbonded, nil
	case "withdrawn":
		return Withdrawn, nil
	case "transitioned":
		return Transitioned, nil
	default:
		return "", fmt.Errorf("invalid delegation state: %s", s)
	}
}

type CovenantSignature struct {
	CovenantBtcPkHex string `bson:"covenant_btc_pk_hex"`
	SignatureHex     string `bson:"signature_hex"`
}

type BTCDelegationCreatedBbnBlock struct {
	Height    int64 `bson:"height"`
	Timestamp int64 `bson:"timestamp"` // epoch time in seconds
}

type BTCDelegationDetails struct {
	StakingTxHashHex            string                       `bson:"_id"` // Primary key
	StakingTxHex                string                       `bson:"staking_tx_hex"`
	StakingTime                 uint32                       `bson:"staking_time"`
	StakingAmount               uint64                       `bson:"staking_amount"`
	StakingOutputIdx            uint32                       `bson:"staking_output_idx"`
	StakerBtcPkHex              string                       `bson:"staker_btc_pk_hex"`
	FinalityProviderBtcPksHex   []string                     `bson:"finality_provider_btc_pks_hex"`
	StartHeight                 uint32                       `bson:"start_height"`
	EndHeight                   uint32                       `bson:"end_height"`
	State                       DelegationState              `bson:"state"`
	ParamsVersion               uint32                       `bson:"params_version"`
	UnbondingTime               uint32                       `bson:"unbonding_time"`
	UnbondingTx                 string                       `bson:"unbonding_tx"`
	CovenantUnbondingSignatures []CovenantSignature          `bson:"covenant_unbonding_signatures"`
	BTCDelegationCreatedBlock   BTCDelegationCreatedBbnBlock `bson:"btc_delegation_created_bbn_block"`
	SlashingTxHex               string                       `bson:"slashing_tx_hex"`           // Will be "" if not slashed
	UnbondingSlashingTxHex      string                       `bson:"unbonding_slashing_tx_hex"` // Will be "" if not slashed
}
