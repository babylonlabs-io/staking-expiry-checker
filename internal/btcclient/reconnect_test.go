package btcclient

import (
	"fmt"
	"testing"
	"time"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
)

func TestReconnect(t *testing.T) {
	// t.Skip("Manual")

	cfg := &config.BTCConfig{
		RPCHost:              "127.0.0.30:18443",
		RPCUser:              "user",
		RPCPass:              "pass",
		BlockPollingInterval: time.Second,
		TxPollingInterval:    time.Second,
		BlockCacheSize:       20 * 1024 * 1024, // 20 MB
		MaxRetryTimes:        1,
		RetryInterval:        time.Millisecond,
		NetParams:            "regtest",
	}
	connCfg, err := cfg.ToConnConfig()
	require.NoError(t, err)

	cl, err := newRPCClientWithReconect(connCfg)
	require.NoError(t, err)

	for i := range 2 {
		fmt.Printf("Iteration %d\n", i)
		count, err := cl.GetBlockCount()
		require.NoError(t, err)
		spew.Dump(count)
	}
}
