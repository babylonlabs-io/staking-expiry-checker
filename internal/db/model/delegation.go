package model

import "github.com/babylonlabs-io/staking-expiry-checker/internal/types"

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
	State                       types.DelegationState        `bson:"state"`
	ParamsVersion               uint32                       `bson:"params_version"`
	UnbondingTime               uint32                       `bson:"unbonding_time"`
	UnbondingTx                 string                       `bson:"unbonding_tx"`
	CovenantUnbondingSignatures []CovenantSignature          `bson:"covenant_unbonding_signatures"`
	BTCDelegationCreatedBlock   BTCDelegationCreatedBbnBlock `bson:"btc_delegation_created_bbn_block"`
	SlashingTxHex               string                       `bson:"slashing_tx_hex"`           // Will be "" if not slashed
	UnbondingSlashingTxHex      string                       `bson:"unbonding_slashing_tx_hex"` // Will be "" if not slashed
}
