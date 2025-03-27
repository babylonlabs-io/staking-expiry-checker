package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/db/model"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	// Hex string lengths for various data
	btcAddressHexLength = 40 // Length for random BTC address hex
	txHashHexLength     = 64 // Length for transaction hash hex
	stakingTxHashLength = 32 // Length for staking transaction hash
	pkHexLength         = 33 // Length for public key hex

	// Time constants
	unbondingMonthsOffset = 6 // Number of months to add for unbonding

	// Random number ranges
	maxOutputIndexRange = 10  // Maximum value for random output index
	txHexMinLength      = 200 // Minimum length for transaction hex
	txHexLengthRange    = 800 // Range for transaction hex length
)

// RandomAlphaNum generates random alphanumeric string
// in case length <= 0 it returns empty string
func RandomAlphaNum(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	if length <= 0 {
		return "", fmt.Errorf("length must be greater than 0")
	}

	randomString := make([]byte, length)
	for i := range randomString {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		randomString[i] = charset[num.Int64()]
	}

	return string(randomString), nil
}

// RandomHex generates a random hex string of the specified length
func RandomHex(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Bitcoin Network Related Generators

// RandomBTCAddress generates a random Bitcoin address for testing
func RandomBTCAddress() (string, error) {
	// Generate a random hex string to simulate a Bitcoin address
	// Real BTC addresses have specific formats, but for testing this is sufficient
	return RandomHex(btcAddressHexLength)
}

// RandomTxHash generates a random Bitcoin transaction hash
// Bitcoin transaction hashes are 32 bytes, displayed as 64 hex characters
func RandomTxHash() (string, error) {
	return RandomHex(txHashHexLength)
}

// RandomBlockHeight generates a random block height within a realistic range
// minHeight and maxHeight parameters allow you to constrain the range
// If they are 0, it uses default ranges
func RandomBlockHeight(minHeight, maxHeight uint64) (uint64, error) {
	// Default to a range of typical recent Bitcoin heights if not specified
	if minHeight == 0 {
		minHeight = 700000 // Roughly early 2022
	}
	if maxHeight == 0 {
		maxHeight = 800000 // Roughly early 2023
	}

	// Ensure valid range
	if maxHeight <= minHeight {
		maxHeight = minHeight + 10000
	}

	range_ := maxHeight - minHeight
	n, err := rand.Int(rand.Reader, big.NewInt(int64(range_)))
	if err != nil {
		return 0, err
	}

	return minHeight + uint64(n.Int64()), nil
}

// RandomTimestamp generates a random Unix timestamp within a realistic range
// minDate and maxDate allow you to constrain the range
// If they are zero values, it uses default ranges
func RandomTimestamp(minDate, maxDate time.Time) (int64, error) {
	// Default to a range of one year ago to now if not specified
	if minDate.IsZero() {
		minDate = time.Now().AddDate(-1, 0, 0)
	}
	if maxDate.IsZero() {
		maxDate = time.Now()
	}

	// Ensure valid range
	if maxDate.Before(minDate) {
		maxDate = minDate.AddDate(0, unbondingMonthsOffset, 0) // Add months
	}

	minUnix := minDate.Unix()
	maxUnix := maxDate.Unix()
	range_ := maxUnix - minUnix

	n, err := rand.Int(rand.Reader, big.NewInt(range_))
	if err != nil {
		return 0, err
	}

	return minUnix + n.Int64(), nil
}

// RandomAmount generates a random Bitcoin amount in satoshis
// minAmount and maxAmount allow you to constrain the range
// If they are 0, it uses default ranges
func RandomAmount(minAmount, maxAmount int64) (int64, error) {
	// Default to a range of 0.01 BTC to 10 BTC if not specified
	if minAmount == 0 {
		minAmount = 1000000 // 0.01 BTC in satoshis
	}
	if maxAmount == 0 {
		maxAmount = 1000000000 // 10 BTC in satoshis
	}

	// Ensure valid range
	if maxAmount <= minAmount {
		maxAmount = minAmount * 10
	}

	range_ := maxAmount - minAmount
	n, err := rand.Int(rand.Reader, big.NewInt(range_))
	if err != nil {
		return 0, err
	}

	return minAmount + n.Int64(), nil
}

// RandomOutputIndex generates a random output index for a transaction
// Bitcoin transactions typically have a small number of outputs
func RandomOutputIndex() (uint64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(maxOutputIndexRange))
	if err != nil {
		return 0, err
	}
	return uint64(n.Int64()), nil
}

// RandomTimelock generates a random timelock value
// Typical values might be multiples of 144 (days in blocks)
func RandomTimelock() (uint64, error) {
	// Generate a timelock between 1 day (144 blocks) and 4 weeks (4032 blocks)
	blocks := []uint64{144, 288, 432, 576, 1008, 1440, 2016, 4032}
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(blocks))))
	if err != nil {
		return 0, err
	}
	return blocks[index.Int64()], nil
}

// RandomTxHex generates a random transaction hex
// Transaction hex strings can be very long, but we'll create a reasonable length for testing
func RandomTxHex() (string, error) {
	// Generate a random length between min-max hex chars
	length, err := rand.Int(rand.Reader, big.NewInt(txHexLengthRange))
	if err != nil {
		return "", err
	}
	length = length.Add(length, big.NewInt(txHexMinLength)) // Add min to get min-max range

	// Generate random bytes
	bytes := make([]byte, length.Int64()/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

// Complex Object Generators

// GenerateRandomDelegation creates a mock delegation for testing
func GenerateRandomDelegation(state types.DelegationState) (*model.DelegationDocument, error) {
	stakingTxHash, err := RandomHex(stakingTxHashLength)
	if err != nil {
		return nil, err
	}

	stakerPk, err := RandomHex(pkHexLength)
	if err != nil {
		return nil, err
	}

	fpPk, err := RandomHex(pkHexLength)
	if err != nil {
		return nil, err
	}

	txHex, err := RandomTxHex()
	if err != nil {
		return nil, err
	}

	// Get random values for timestamps and heights
	currentHeight, err := RandomBlockHeight(0, 0)
	if err != nil {
		return nil, err
	}

	currentTime, err := RandomTimestamp(time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}

	timelock, err := RandomTimelock()
	if err != nil {
		return nil, err
	}

	stakingTx := &model.TimelockTransaction{
		TxHex:          txHex,
		OutputIndex:    0,
		StartTimestamp: currentTime - 3600*24*14, // 14 days ago
		StartHeight:    currentHeight - 2016,     // ~2 weeks ago in blocks
		TimeLock:       timelock,
	}

	delegation := &model.DelegationDocument{
		StakingTxHashHex:      stakingTxHash,
		StakerPkHex:           stakerPk,
		FinalityProviderPkHex: fpPk,
		State:                 state,
		StakingTx:             stakingTx,
	}

	// Add unbonding tx if in unbonding state
	if state == types.Unbonding || state == types.Unbonded {
		unbondingTxHex, err := RandomTxHex()
		if err != nil {
			return nil, err
		}

		unbondingTimelock, err := RandomTimelock()
		if err != nil {
			return nil, err
		}

		delegation.UnbondingTx = &model.TimelockTransaction{
			TxHex:          unbondingTxHex,
			OutputIndex:    0,
			StartTimestamp: currentTime - 3600*24*7, // 7 days ago
			StartHeight:    currentHeight - 1008,    // ~1 week ago in blocks
			TimeLock:       unbondingTimelock,
		}
	}

	return delegation, nil
}

// GenerateRandomTimeLockDocument creates a mock TimeLockDocument for testing
func GenerateRandomTimeLockDocument(expiryHeight uint64, txType string) (*model.TimeLockDocument, error) {
	stakingTxHash, err := RandomTxHash()
	if err != nil {
		return nil, err
	}

	doc := &model.TimeLockDocument{
		ID:               primitive.NewObjectID(),
		StakingTxHashHex: stakingTxHash,
		ExpireHeight:     expiryHeight,
		TxType:           txType,
	}

	return doc, nil
}
