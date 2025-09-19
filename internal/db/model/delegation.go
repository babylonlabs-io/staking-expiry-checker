package model

import "github.com/babylonlabs-io/staking-expiry-checker/internal/types"

type TimelockTransaction struct {
	TxHex          string `bson:"tx_hex"`
	OutputIndex    uint64 `bson:"output_index"`
	StartTimestamp int64  `bson:"start_timestamp"`
	StartHeight    uint64 `bson:"start_height"`
	TimeLock       uint64 `bson:"timelock"`
}

type DelegationDocument struct {
	StakingTxHashHex      string                `bson:"_id"` // Primary key
	StakerPkHex           string                `bson:"staker_pk_hex"`
	FinalityProviderPkHex string                `bson:"finality_provider_pk_hex"`
	StakingValue          uint64                `bson:"staking_value"` // Added for stats calculation
	State                 types.DelegationState `bson:"state"`
	StakingTx             *TimelockTransaction  `bson:"staking_tx"` // Always exist
	UnbondingTx           *TimelockTransaction  `bson:"unbonding_tx,omitempty"`
	IsOverflow            bool                  `bson:"is_overflow"` // Added for stats filtering
}

type DelegationScanPagination struct {
	StakingTxHashHex string `json:"staking_tx_hash_hex"`
}

func BuildDelegationScanPaginationToken(d DelegationDocument) (string, error) {
	page := &DelegationScanPagination{
		StakingTxHashHex: d.StakingTxHashHex,
	}
	token, err := GetPaginationToken(page)
	if err != nil {
		return "", err
	}
	return token, nil
}

// V1OverallStatsDocument represents the overall stats document
type V1OverallStatsDocument struct {
	Id                string `bson:"_id"`
	ActiveTvl         int64  `bson:"active_tvl"`
	ActiveDelegations int64  `bson:"active_delegations"`
}
