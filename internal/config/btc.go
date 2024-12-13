package config

import (
	"fmt"
	"time"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/utils"
)

type BtcConfig struct {
	// Endpoint specifies the URL of the Bitcoin RPC server without the protocol prefix (http:// or https://).
	Endpoint string `mapstructure:"endpoint"`
	/*
		DisableTLS controls the request protocol used for communication.
		When true, connections use HTTP. When false, HTTPS is used for secure communication.
	*/
	DisableTLS bool `mapstructure:"disable-tls"`
	// NetParams defines the network parameters (e.g., mainnet, testnet & signet).
	NetParams string `mapstructure:"net-params"`
	// RpcUser is the username for RPC server authentication.
	RpcUser string `mapstructure:"rpc-user"`
	// RpcPass is the password for RPC server authentication.
	RpcPass string `mapstructure:"rpc-pass"`

	// PrunedNodeMaxPeers is the maximum number of peers to connect to when using a pruned node.
	PrunedNodeMaxPeers int `mapstructure:"prunednodemaxpeers"`
	// BlockPollingInterval is the interval at which to poll for new blocks.
	BlockPollingInterval time.Duration `mapstructure:"blockpollinginterval"`
	// TxPollingInterval is the interval at which to poll for new transactions.
	TxPollingInterval time.Duration `mapstructure:"txpollinginterval"`
	// TxPollingIntervalJitter is the jitter factor for the transaction polling interval.
	TxPollingIntervalJitter float64 `mapstructure:"txpollingintervaljitter"`
	// BlockCacheSize is the size of the block cache.
	BlockCacheSize uint64        `mapstructure:"blockcachesize"`
	MaxRetryTimes  uint          `mapstructure:"maxretrytimes"`
	RetryInterval  time.Duration `mapstructure:"retryinterval"`
}

func (cfg *BtcConfig) Validate() error {
	if _, ok := utils.GetValidNetParams()[cfg.NetParams]; !ok {
		return fmt.Errorf("invalid net params: %v", cfg.NetParams)
	}

	return nil
}
