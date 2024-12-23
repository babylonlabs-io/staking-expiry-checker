package btcclient

import (
	"github.com/btcsuite/btcd/rpcclient"

	"github.com/babylonlabs-io/staking-expiry-checker/internal/config"
	"github.com/babylonlabs-io/staking-expiry-checker/internal/observability/metrics"
)

type BtcClient struct {
	client *rpcclient.Client
	cfg    *config.BTCConfig
}

func NewBtcClient(cfg *config.BTCConfig) (*BtcClient, error) {
	connCfg, err := cfg.ToConnConfig()
	if err != nil {
		return nil, err
	}

	rpcClient, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}

	return &BtcClient{
		client: rpcClient,
		cfg:    cfg,
	}, nil
}

func (b *BtcClient) GetBlockCount() (int64, error) {
	return metrics.RecordBtcClientMetrics[int64](b.client.GetBlockCount)
}

func (b *BtcClient) GetBlockTimestamp(height uint64) (int64, error) {
	return metrics.RecordBtcClientMetrics[int64](func() (int64, error) {
		hash, err := b.client.GetBlockHash(int64(height))
		if err != nil {
			return 0, err
		}

		header, err := b.client.GetBlockHeader(hash)
		if err != nil {
			return 0, err
		}

		return header.Timestamp.Unix(), nil
	})
}
