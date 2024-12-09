package types

import "fmt"

type TransactionType string

const (
	// Refer to natural timelock expired staking transaction
	TransactionTypeActive TransactionType = "active"
	// Refer to early unbonding of staking transaction
	TransactionTypeUnbonding TransactionType = "unbonding"
)

func (t TransactionType) ToString() string {
	return string(t)
}

func FromString(s string) (TransactionType, error) {
	switch s {
	case string(TransactionTypeActive):
		return TransactionTypeActive, nil
	case string(TransactionTypeUnbonding):
		return TransactionTypeUnbonding, nil
	}
	return "", fmt.Errorf("invalid transaction type: %s", s)
}
