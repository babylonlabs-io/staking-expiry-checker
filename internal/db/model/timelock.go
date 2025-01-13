package model

import "go.mongodb.org/mongo-driver/bson/primitive"

type TimeLockDocument struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	StakingTxHashHex string             `bson:"staking_tx_hash_hex"`
	ExpireHeight     uint64             `bson:"expire_height"`
	TxType           string             `bson:"tx_type"`
}

func NewTimeLockDocument(
	stakingTxHashHex string, expireHeight uint64, txType string,
) TimeLockDocument {
	return TimeLockDocument{
		StakingTxHashHex: stakingTxHashHex,
		ExpireHeight:     expireHeight,
		TxType:           txType,
	}
}
