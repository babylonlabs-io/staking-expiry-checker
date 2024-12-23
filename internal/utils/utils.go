package utils

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
)

type SupportedBtcNetwork string

const (
	BtcMainnet SupportedBtcNetwork = "mainnet"
	BtcTestnet SupportedBtcNetwork = "testnet"
	BtcSimnet  SupportedBtcNetwork = "simnet"
	BtcRegtest SupportedBtcNetwork = "regtest"
	BtcSignet  SupportedBtcNetwork = "signet"
)

func (c SupportedBtcNetwork) String() string {
	return string(c)
}

func GetBTCParams(net string) (*chaincfg.Params, error) {
	switch net {
	case BtcMainnet.String():
		return &chaincfg.MainNetParams, nil
	case BtcTestnet.String():
		return &chaincfg.TestNet3Params, nil
	case BtcSimnet.String():
		return &chaincfg.SimNetParams, nil
	case BtcRegtest.String():
		return &chaincfg.RegressionNetParams, nil
	case BtcSignet.String():
		return &chaincfg.SigNetParams, nil
	}
	return nil, fmt.Errorf("BTC network with name %s does not exist. should be one of {%s, %s, %s, %s, %s}",
		net, BtcMainnet.String(), BtcTestnet.String(), BtcSimnet.String(), BtcRegtest.String(), BtcSignet.String())
}

func GetValidNetParams() map[string]bool {
	params := map[string]bool{
		BtcMainnet.String(): true,
		BtcTestnet.String(): true,
		BtcSimnet.String():  true,
		BtcRegtest.String(): true,
		BtcSignet.String():  true,
	}

	return params
}

// GetFunctionName retrieves the name of the function at the specified call depth.
// depth 0 = getFunctionName, depth 1 = caller of getFunctionName, depth 2 = caller of that caller, etc.
func GetFunctionName(depth int) string {
	pc, _, _, ok := runtime.Caller(depth + 1) // +1 to account for calling getFunctionName itself
	if !ok {
		return "unknown"
	}

	fullFunctionName := runtime.FuncForPC(pc).Name()
	// Optionally, clean up the function name to get the short form
	shortFunctionName := shortFuncName(fullFunctionName)

	return shortFunctionName
}

// shortFuncName takes the fully qualified function name and returns a shorter version
// by trimming the package path and leaving only the function's name.
func shortFuncName(fullName string) string {
	// Function names include the path to the package, so we trim everything up to the last '/'
	if idx := strings.LastIndex(fullName, "/"); idx >= 0 {
		fullName = fullName[idx+1:]
	}
	// In case the function is a method of a struct, remove the package name as well
	if idx := strings.Index(fullName, "."); idx >= 0 {
		fullName = fullName[idx+1:]
	}
	return fullName
}

func DeserializeBtcTransactionFromHex(txHex string) (*wire.MsgTx, error) {
	// First decode the hex string into bytes
	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string: %w", err)
	}

	// Then deserialize the bytes into a transaction
	reader := bytes.NewReader(txBytes)
	tx := wire.NewMsgTx(wire.TxVersion)
	if err := tx.Deserialize(reader); err != nil {
		return nil, fmt.Errorf("failed to deserialize transaction: %w", err)
	}
	return tx, nil
}

// Contains checks if a slice contains a specific element.
// It uses type parameters to work with any slice type.
func Contains[T comparable](slice []T, element T) bool {
	for _, item := range slice {
		if item == element {
			return true
		}
	}
	return false
}

// push msg to channel c, or quit if quit channel is closed
func PushOrQuit[T any](c chan<- T, msg T, quit <-chan struct{}) {
	select {
	case c <- msg:
	case <-quit:
	}
}

func SerializeBtcTransaction(tx *wire.MsgTx) (string, error) {
	var txBuf bytes.Buffer
	if err := tx.Serialize(&txBuf); err != nil {
		return "", err
	}
	return hex.EncodeToString(txBuf.Bytes()), nil
}
